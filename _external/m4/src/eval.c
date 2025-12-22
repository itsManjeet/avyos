/* GNU m4 -- A simple macro processor

   Copyright (C) 1989-1994, 2006-2007, 2009-2014, 2016-2017, 2020-2025
   Free Software Foundation, Inc.

   This file is part of GNU M4.

   GNU M4 is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   GNU M4 is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

/* This file contains the functions to evaluate integer expressions for
   the "eval" macro.  It is a little, fairly self-contained module, with
   its own scanner, and a recursive descent parser.  The only entry point
   is evaluate ().  */

#include "m4.h"

/* Evaluates token types.  */

typedef enum eval_token
{
  /* Value / 10 is precedence order.  */
  ERROR = 0,
  BADOP,
  EOTEXT,
  LEFTP,
  RIGHTP,
  LNOT,
  NOT,
  NUMBER,
  LOR = 10,
  LAND = 20,
  OR = 30,
  XOR = 40,
  AND = 50,
  ASSIGN = 60,                  /* deprecated synonym to EQ */
  EQ,
  NOTEQ,
  GT = 70,
  GTEQ,
  LS,
  LSEQ,
  LSHIFT = 80,
  RSHIFT,
  PLUS = 90,                    /* precedence for binary op; also serves as a unary op */
  MINUS,                        /* precedence for binary op; also serves as a unary op */
  TIMES = 100,
  DIVIDE,
  MODULO,
  EXPONENT = 110
}
eval_token;

/* Error types.  */

typedef enum eval_error
{
  NO_ERROR,
  DIVIDE_ZERO,
  MODULO_ZERO,
  NEGATIVE_EXPONENT,
  /* All errors prior to SYNTAX_ERROR can be ignored in a dead
     branch of && and ||.  All errors after are just more details
     about a syntax error.  */
  SYNTAX_ERROR,
  MISSING_RIGHT,
  UNKNOWN_INPUT,
  EXCESS_INPUT,
  INVALID_OPERATOR
}
eval_error;

static eval_error primary (int32_t *);
static eval_error parse_expr (int32_t *, eval_error, unsigned);

/*--------------------.
| Lexical functions.  |
`--------------------*/

/* Pointer to next character of input text.  */
static const char *eval_text;

/* Value of eval_text, from before last call of eval_lex ().  This is so we
   can back up, if we have read too much, good for one token lookahead.  */
static const char *last_text;

static void
eval_init_lex (const char *text)
{
  eval_text = text;
  last_text = NULL;
}

static void
eval_undo (void)
{
  eval_text = last_text;
}

/* VAL is numerical value, if any.  */

static eval_token
eval_lex (int32_t *val)
{
  while (c_isspace (*eval_text))
    eval_text++;

  last_text = eval_text;

  if (*eval_text == '\0')
    return EOTEXT;

  if (c_isdigit (*eval_text))
    {
      unsigned int base, digit;
      /* The documentation says that "overflow silently results in wraparound".
         Therefore use an unsigned integer type to avoid undefined behaviour
         when parsing '-2147483648'.  */
      uint32_t value;

      if (*eval_text == '0')
        {
          eval_text++;
          switch (*eval_text)
            {
            case 'x':
            case 'X':
              base = 16;
              eval_text++;
              break;

            case 'b':
            case 'B':
              base = 2;
              eval_text++;
              break;

            case 'r':
            case 'R':
              base = 0;
              eval_text++;
              while (c_isdigit (*eval_text) && base <= 36)
                base = 10 * base + *eval_text++ - '0';
              if (base == 0 || base > 36 || *eval_text != ':')
                return ERROR;
              eval_text++;
              break;

            default:
              base = 8;
            }
        }
      else
        base = 10;

      value = 0;
      for (; *eval_text; eval_text++)
        {
          if (c_isdigit (*eval_text))
            digit = *eval_text - '0';
          else if (c_islower (*eval_text))
            digit = *eval_text - 'a' + 10;
          else if (c_isupper (*eval_text))
            digit = *eval_text - 'A' + 10;
          else
            break;

          if (base == 1)
            {
              if (digit == 1)
                value++;
              else if (digit == 0 && value == 0)
                continue;
              else
                break;
            }
          else if (digit >= base)
            break;
          else
            value = value * base + digit;
        }
      *val = value;
      return NUMBER;
    }

  switch (*eval_text++)
    {
    case '+':
      if (*eval_text == '+' || *eval_text == '=')
        return BADOP;
      return PLUS;
    case '-':
      if (*eval_text == '-' || *eval_text == '=')
        return BADOP;
      return MINUS;
    case '*':
      if (*eval_text == '*')
        {
          eval_text++;
          return EXPONENT;
        }
      else if (*eval_text == '=')
        return BADOP;
      return TIMES;
    case '/':
      if (*eval_text == '=')
        return BADOP;
      return DIVIDE;
    case '%':
      if (*eval_text == '=')
        return BADOP;
      return MODULO;
    case '=':
      if (*eval_text == '=')
        {
          eval_text++;
          return EQ;
        }
      return ASSIGN;
    case '!':
      if (*eval_text == '=')
        {
          eval_text++;
          return NOTEQ;
        }
      return LNOT;
    case '>':
      if (*eval_text == '=')
        {
          eval_text++;
          return GTEQ;
        }
      else if (*eval_text == '>')
        {
          if (*++eval_text == '=')
            return BADOP;
          return RSHIFT;
        }
      return GT;
    case '<':
      if (*eval_text == '=')
        {
          eval_text++;
          return LSEQ;
        }
      else if (*eval_text == '<')
        {
          if (*++eval_text == '=')
            return BADOP;
          return LSHIFT;
        }
      return LS;
    case '^':
      if (*eval_text == '=')
        return BADOP;
      return XOR;
    case '~':
      return NOT;
    case '&':
      if (*eval_text == '&')
        {
          eval_text++;
          return LAND;
        }
      else if (*eval_text == '=')
        return BADOP;
      return AND;
    case '|':
      if (*eval_text == '|')
        {
          eval_text++;
          return LOR;
        }
      else if (*eval_text == '=')
        return BADOP;
      return OR;
    case '(':
      return LEFTP;
    case ')':
      return RIGHTP;
    default:
      return ERROR;
    }
}

/*-----------------------------------------------------.
| Operator precedence parser (based on Pratt parser).  |
`-----------------------------------------------------*/

/* Parse `(expr)', unary operators, and numbers.  */
static eval_error
primary (int32_t *v1)
{
  eval_error er;
  int32_t v2;

  switch (eval_lex (v1))
    {
      /* Number */
    case NUMBER:
      return NO_ERROR;

      /* Parenthesis */
    case LEFTP:
      er = primary (v1);
      er = parse_expr (v1, er, 1);
      if (er >= SYNTAX_ERROR)
        return er;
      switch (eval_lex (&v2))
        {
        case ERROR:
          return UNKNOWN_INPUT;
        case RIGHTP:
          return er;
        default:
          return MISSING_RIGHT;
        }

      /* Unary operators */
      /* Minimize undefined C behavior on overflow.  This code assumes
         that the implementation-defined overflow when casting
         unsigned to signed is a silent twos-complement
         wrap-around.  */
    case PLUS:
      return primary (v1);
    case MINUS:
      er = primary (v1);
      *v1 = (int32_t) -(uint32_t) *v1;
      return er;
    case NOT:
      er = primary (v1);
      *v1 = ~*v1;
      return er;
    case LNOT:
      er = primary (v1);
      *v1 = *v1 == 0 ? 1 : 0;
      return er;

      /* Anything else */
    case ERROR:
      return UNKNOWN_INPUT;
    case BADOP:
      return INVALID_OPERATOR;
    default:
      return SYNTAX_ERROR;
    }
}

/* Parse binary operators with at least MIN_PREC precedence.  */
static eval_error
parse_expr (int32_t *v1, eval_error er, unsigned min_prec)
{
  eval_token et;
  eval_token et2;
  eval_error er2;
  int32_t v2;
  int32_t v3;
  uint32_t u1;

  if (er >= SYNTAX_ERROR)
    return er;
  et = eval_lex (&v2);
  while (et / 10 >= min_prec)
    {
      if ((er2 = primary (&v2)) >= SYNTAX_ERROR)
        return er2;
      et2 = eval_lex (&v3);
      /* Handle binary operators of higher precedence or right-associativity */
      while (et2 / 10 > et / 10 || et2 == EXPONENT)
        {
          eval_undo ();
          if ((er2 = parse_expr (&v2, er2, et2 / 10)) >= SYNTAX_ERROR)
            return er2;
          et2 = eval_lex (&v3);
        }
      /* Reduce the two values by the given binary operator */
      switch (et)
        {
        case EXPONENT:
          /* Minimize undefined C behavior on overflow.  This code assumes
             that the implementation-defined overflow when casting
             unsigned to signed is a silent twos-complement
             wrap-around.  */
          u1 = 1;
          if (v2 < 0)
            er = NEGATIVE_EXPONENT;
          else if (*v1 == 0 && v2 == 0)
            er = DIVIDE_ZERO;
          else
            {
              while (v2-- > 0)
                u1 *= (uint32_t) *v1;
            }
          *v1 = u1;
          break;

        case TIMES:
          *v1 = (int32_t) ((uint32_t) *v1 * (uint32_t) v2);
          break;
        case DIVIDE:
          if (v2 == 0)
            er = DIVIDE_ZERO;
          else if (v2 == -1)
            /* Avoid overflow, and the x86 SIGFPE on INT_MIN / -1.  */
            *v1 = (int32_t) -(uint32_t) *v1;
          else
            *v1 /= v2;
          break;
        case MODULO:
          if (v2 == 0)
            er = MODULO_ZERO;
          else if (v2 == -1)
            /* Avoid the x86 SIGFPE on INT_MIN % -1.  */
            *v1 = 0;
          else
            *v1 %= v2;
          break;

        case PLUS:
          *v1 = (int32_t) ((uint32_t) *v1 + (uint32_t) v2);
          break;
        case MINUS:
          *v1 = (int32_t) ((uint32_t) *v1 - (uint32_t) v2);
          break;

        case LSHIFT:
          u1 = *v1;
          u1 <<= (uint32_t) (v2 & 0x1f);
          *v1 = u1;
          break;
        case RSHIFT:
          u1 = *v1 < 0 ? ~*v1 : *v1;
          u1 >>= (uint32_t) (v2 & 0x1f);
          *v1 = *v1 < 0 ? ~u1 : u1;
          break;

        case GT:
          *v1 = *v1 > v2;
          break;
        case GTEQ:
          *v1 = *v1 >= v2;
          break;
        case LS:
          *v1 = *v1 < v2;
          break;
        case LSEQ:
          *v1 = *v1 <= v2;
          break;

        case ASSIGN:
          M4ERROR ((warning_status, 0, _("\
Warning: recommend ==, not =, for equality operator")));
          FALLTHROUGH;
        case EQ:
          *v1 = *v1 == v2;
          break;
        case NOTEQ:
          *v1 = *v1 != v2;
          break;

        case AND:
          *v1 &= v2;
          break;

        case XOR:
          *v1 ^= v2;
          break;

        case OR:
          *v1 |= v2;
          break;

          /* Implement short-circuiting of valid syntax.  */
        case LAND:
          if (!*v1)
            er2 = NO_ERROR;
          *v1 = *v1 && v2;
          break;

        case LOR:
          if (*v1)
            er2 = NO_ERROR;
          *v1 = *v1 || v2;
          break;

        default:
          M4ERROR ((warning_status, 0,
                    "INTERNAL ERROR: unexpected operator in evaluate ()"));
          abort ();
        }
      if (er == NO_ERROR)
        er = er2;
      et = et2;
    }

  eval_undo ();
  return er;
}

/*---------------------------------------.
| Main entry point, called from "eval".  |
`---------------------------------------*/

bool
evaluate (const char *expr, int32_t *val)
{
  eval_error err;

  eval_init_lex (expr);
  err = primary (val);
  err = parse_expr (val, err, 1);

  if (err == NO_ERROR && *eval_text != '\0')
    {
      if (eval_lex (val) == BADOP)
        err = INVALID_OPERATOR;
      else
        err = EXCESS_INPUT;
    }

  switch (err)
    {
    case NO_ERROR:
      break;

    case MISSING_RIGHT:
      M4ERROR ((warning_status, 0,
                _("bad expression in eval (missing right parenthesis): %s"),
                expr));
      break;

    case SYNTAX_ERROR:
      M4ERROR ((warning_status, 0, _("bad expression in eval: %s"), expr));
      break;

    case UNKNOWN_INPUT:
      M4ERROR ((warning_status, 0,
                _("bad expression in eval (bad input): %s"), expr));
      break;

    case EXCESS_INPUT:
      M4ERROR ((warning_status, 0,
                _("bad expression in eval (excess input): %s"), expr));
      break;

    case INVALID_OPERATOR:
      M4ERROR ((warning_status, 0, _("invalid operator in eval: %s"), expr));
      retcode = EXIT_FAILURE;
      break;

    case DIVIDE_ZERO:
      M4ERROR ((warning_status, 0, _("divide by zero in eval: %s"), expr));
      break;

    case MODULO_ZERO:
      M4ERROR ((warning_status, 0, _("modulo by zero in eval: %s"), expr));
      break;

    case NEGATIVE_EXPONENT:
      M4ERROR ((warning_status, 0, _("negative exponent in eval: %s"), expr));
      break;

    default:
      M4ERROR ((warning_status, 0,
                "INTERNAL ERROR: bad error code in evaluate ()"));
      abort ();
    }

  return err != NO_ERROR;
}
