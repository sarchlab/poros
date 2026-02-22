void kernel_deriche(int w, int h, float alpha_f,
    double *imgIn, double *imgOut, double *y1, double *y2) {
  double alpha = (double)alpha_f;

  // Precomputed constants (avoid exp/pow library calls).
  // For alpha=0.25: k = -0.18869340542894975
  double kema = -0.18869340542894975 * 0.7788007830714049; // k * exp(-alpha)
  double a1 = -0.18869340542894975;
  double a2 = kema * (alpha - 1.0);
  double a3 = kema * (alpha + 1.0);
  double a4 = 0.18869340542894975 * 0.6065306597633104; // -k * exp(-2*alpha)
  double b1 = 0.8408964152537145;  // pow(2, -alpha)
  double b2 = -0.6065306597633104; // -exp(-2*alpha)

  double ym1, ym2, xm1;
  double yp1, yp2, xp1, xp2;
  double tm1, tp1, tp2;

  for (int i = 0; i < w; i++) {
    ym1 = 0.0; ym2 = 0.0; xm1 = 0.0;
    for (int j = 0; j < h; j++) {
      y1[i*h+j] = a1*imgIn[i*h+j] + a2*xm1 + b1*ym1 + b2*ym2;
      xm1 = imgIn[i*h+j];
      ym2 = ym1;
      ym1 = y1[i*h+j];
    }
  }

  for (int i = 0; i < w; i++) {
    yp1 = 0.0; yp2 = 0.0; xp1 = 0.0; xp2 = 0.0;
    for (int j = h-1; j >= 0; j--) {
      y2[i*h+j] = a3*xp1 + a4*xp2 + b1*yp1 + b2*yp2;
      xp2 = xp1;
      xp1 = imgIn[i*h+j];
      yp2 = yp1;
      yp1 = y2[i*h+j];
    }
  }

  for (int i = 0; i < w; i++)
    for (int j = 0; j < h; j++)
      imgOut[i*h+j] = y1[i*h+j] + y2[i*h+j];

  for (int j = 0; j < h; j++) {
    tm1 = 0.0; ym1 = 0.0; ym2 = 0.0;
    for (int i = 0; i < w; i++) {
      y1[i*h+j] = a1*imgOut[i*h+j] + a2*tm1 + b1*ym1 + b2*ym2;
      tm1 = imgOut[i*h+j];
      ym2 = ym1;
      ym1 = y1[i*h+j];
    }
  }

  for (int j = 0; j < h; j++) {
    tp1 = 0.0; tp2 = 0.0; yp1 = 0.0; yp2 = 0.0;
    for (int i = w-1; i >= 0; i--) {
      y2[i*h+j] = a3*tp1 + a4*tp2 + b1*yp1 + b2*yp2;
      tp2 = tp1;
      tp1 = imgOut[i*h+j];
      yp2 = yp1;
      yp1 = y2[i*h+j];
    }
  }

  for (int i = 0; i < w; i++)
    for (int j = 0; j < h; j++)
      imgOut[i*h+j] = y1[i*h+j] + y2[i*h+j];
}
