#pragma once

#include <stdint.h>
#include <uchar.h>

struct composed {
    char32_t *chars;
    struct composed *left;
    struct composed *right;
    uint32_t key;
    uint8_t count;
    uint8_t width;
    uint8_t forced_width;
};

uint32_t composed_key_from_chars(const uint32_t chars[], size_t count);
uint32_t composed_key_from_key(uint32_t prev_key, uint32_t next_char);

const struct composed *composed_lookup(struct composed *root, uint32_t key);
const struct composed *composed_lookup_without_collision(
    struct composed *root, uint32_t *key,
    const char32_t *prefix, size_t prefix_len, char32_t wc, int forced_width);
void composed_insert(struct composed **root, struct composed *node);

void composed_free(struct composed *root);
