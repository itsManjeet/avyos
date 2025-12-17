#include "hsl.h"

#include <math.h>

uint32_t
hsl_to_rgb(int hue, int sat, int lum)
{
    double L = lum / 100.0;
    double S = sat / 100.0;
    double C = (1. - fabs(2. * L - 1.)) * S;

    double X = C * (1. - fabs(fmod((double)hue / 60., 2.) - 1.));
    double m = L - C / 2.;

    double r, g, b;
    if (hue >= 0 && hue <= 60) {
        r = C;
        g = X;
        b = 0.;
    } else if (hue >= 60 && hue <= 120) {
        r = X;
        g = C;
        b = 0.;
    } else if (hue >= 120 && hue <= 180) {
        r = 0.;
        g = C;
        b = X;
    } else if (hue >= 180 && hue <= 240) {
        r = 0.;
        g = X;
        b = C;
    } else if (hue >= 240 && hue <= 300) {
        r = X;
        g = 0.;
        b = C;
    } else if (hue >= 300 && hue <= 360) {
        r = C;
        g = 0.;
        b = X;
    } else {
        r = 0.;
        g = 0.;
        b = 0.;
    }

    r += m;
    g += m;
    b += m;

    return (
        (uint8_t)round(r * 255.) << 16 |
        (uint8_t)round(g * 255.) << 8 |
        (uint8_t)round(b * 255.) << 0);
}
