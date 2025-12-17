#include "composed.h"

#include <stdlib.h>
#include <stdbool.h>

#include "debug.h"
#include "terminal.h"

uint32_t
composed_key_from_chars(const uint32_t chars[], size_t count)
{
    if (count == 0)
        return 0;

    uint32_t key = chars[0];
    for (size_t i = 1; i < count; i++)
        key = composed_key_from_key(key, chars[i]);

    return key;
}

uint32_t
composed_key_from_key(uint32_t prev_key, uint32_t next_char)
{
    unsigned bits = 32 - __builtin_clz(CELL_COMB_CHARS_HI - CELL_COMB_CHARS_LO);

    /* Rotate old key 8 bits */
    uint32_t new_key = (prev_key << 8) | (prev_key >> (bits - 8));

    /* xor with new char */
    new_key ^= next_char;

    /* Multiply with magic hash constant */
    new_key *= 2654435761ul;

    /* And mask, to ensure the new value is within range */
    new_key &= CELL_COMB_CHARS_HI - CELL_COMB_CHARS_LO;
    return new_key;
}

UNITTEST
{
    const char32_t chars[] = U"abcdef";

    uint32_t k1 = composed_key_from_key(chars[0], chars[1]);
    uint32_t k2 = composed_key_from_chars(chars, 2);
    xassert(k1 == k2);

    uint32_t k3 = composed_key_from_key(k2, chars[2]);
    uint32_t k4 = composed_key_from_chars(chars, 3);
    xassert(k3 == k4);
}

const struct composed *
composed_lookup(struct composed *root, uint32_t key)
{
    struct composed *node = root;

    while (node != NULL) {
        if (key == node->key)
            return node;

        node = key < node->key ? node->left : node->right;
    }

    return NULL;
}

const struct composed *
composed_lookup_without_collision(struct composed *root, uint32_t *key,
                                  const char32_t *prefix_text, size_t prefix_len,
                                  char32_t wc, int forced_width)
{
    while (true) {
        const struct composed *cc = composed_lookup(root, *key);
        if (cc == NULL)
            return NULL;

        bool match = cc->count == prefix_len + 1 &&
                     cc->forced_width == forced_width &&
                     cc->chars[prefix_len] == wc;

        if (match) {
            for (size_t i = 0; i < prefix_len; i++) {
                if (cc->chars[i] != prefix_text[i]) {
                    match = false;
                    break;
                }
            }
        }

        if (match)
            return cc;

        (*key)++;
        *key &= CELL_COMB_CHARS_HI - CELL_COMB_CHARS_LO;

        /* TODO: this will loop infinitely if the composed table is full */
    }

    return NULL;
}

void
composed_insert(struct composed **root, struct composed *node)
{
    node->left = node->right = NULL;

    if (*root == NULL) {
        *root = node;
        return;
    }

    uint32_t key = node->key;

    struct composed *prev = NULL;
    struct composed *n = *root;

    while (n != NULL) {
        xassert(n->key != node->key);

        prev = n;
        n = key < n->key ? n->left : n->right;
    }

    xassert(prev != NULL);
    xassert(n == NULL);

    if (key < prev->key) {
        xassert(prev->left == NULL);
        prev->left = node;
    } else {
        xassert(prev->right == NULL);
        prev->right = node;
    }
}

void
composed_free(struct composed *root)
{
    if (root == NULL)
        return;

    composed_free(root->left);
    composed_free(root->right);

    free(root->chars);
    free(root);
}
