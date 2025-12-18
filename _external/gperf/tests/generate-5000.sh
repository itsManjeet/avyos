#!/bin/sh
clisp -q -x '(dotimes (i 5000) (write (random 10000000000000000) :base 36) (terpri))'
