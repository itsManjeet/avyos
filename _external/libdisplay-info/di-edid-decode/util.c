#include "di-edid-decode.h"

static int
gcd(int a, int b)
{
	int tmp;

	while (b) {
		tmp = b;
		b = a % b;
		a = tmp;
	}

	return a;
}

void
compute_aspect_ratio(int width, int height, int *horiz_ratio, int *vert_ratio)
{
	int d;

	d = gcd(width, height);
	if (d == 0) {
		*horiz_ratio = *vert_ratio = 0;
	} else {
		*horiz_ratio = width / d;
		*vert_ratio = height / d;
	}

	if (*horiz_ratio == 8 && *vert_ratio == 5) {
		*horiz_ratio = 16;
		*vert_ratio = 10;
	}
}
