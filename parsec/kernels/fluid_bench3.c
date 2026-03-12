#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <mach/mach_time.h>

void sph_compute(
    double *px, double *py, double *pz,
    double *vx, double *vy, double *vz,
    double *density, double *pressure,
    double *fx, double *fy, double *fz,
    int n, double h, double mass, double rho0, double k, double mu
) {
    double h2 = h * h;
    double poly6_coeff = 315.0 / (64.0 * 3.141592653589793 * h*h*h*h*h*h*h*h*h);
    double spiky_coeff = -45.0 / (3.141592653589793 * h*h*h*h*h*h);
    double visc_coeff = 45.0 / (3.141592653589793 * h*h*h*h*h*h);
    for (int i = 0; i < n; i++) {
        double rho = 0.0;
        for (int j = 0; j < n; j++) {
            double dx = px[i]-px[j], dy = py[i]-py[j], dz = pz[i]-pz[j];
            double r2 = dx*dx + dy*dy + dz*dz;
            if (r2 < h2) { double d = h2-r2; rho += mass * poly6_coeff * d*d*d; }
        }
        density[i] = rho; pressure[i] = k*(rho-rho0);
    }
    for (int i = 0; i < n; i++) {
        double fxi=0,fyi=0,fzi=0;
        for (int j = 0; j < n; j++) {
            if (i==j) continue;
            double dx=px[i]-px[j], dy=py[i]-py[j], dz=pz[i]-pz[j];
            double r2=dx*dx+dy*dy+dz*dz;
            if (r2<h2 && r2>1e-12) {
                double r=__builtin_sqrt(r2), d=h-r;
                double pt=-mass*(pressure[i]+pressure[j])/(2.0*density[j]);
                double fp=spiky_coeff*d*d*pt/r;
                fxi+=fp*dx; fyi+=fp*dy; fzi+=fp*dz;
                double vt=mu*mass/density[j], fv=visc_coeff*d*vt;
                fxi+=fv*(vx[j]-vx[i]); fyi+=fv*(vy[j]-vy[i]); fzi+=fv*(vz[j]-vz[i]);
            }
        }
        fx[i]=fxi; fy[i]=fyi; fz[i]=fzi;
    }
}

int main() {
    int sizes[] = {16, 32, 48, 64};
    mach_timebase_info_data_t info;
    mach_timebase_info(&info);
    
    for (int si = 0; si < 4; si++) {
        int n = sizes[si];
        double *px=calloc(n,8),*py=calloc(n,8),*pz=calloc(n,8);
        double *vx=calloc(n,8),*vy=calloc(n,8),*vz=calloc(n,8);
        double *den=calloc(n,8),*pres=calloc(n,8);
        double *fx=calloc(n,8),*fy=calloc(n,8),*fz=calloc(n,8);
        int sq = (int)ceil(sqrt((double)n));
        for(int i=0;i<n;i++){px[i]=(i%sq)*0.3;py[i]=(i/sq)*0.3;vx[i]=0.1*(1-2*(i%2));vy[i]=0.1*(1-2*((i/2)%2));}
        
        // Warm up
        int W = n < 32 ? 100000 : (n < 64 ? 10000 : 1000);
        for(int i=0;i<W;i++){
            for(int j=0;j<n;j++){den[j]=0;pres[j]=0;fx[j]=0;fy[j]=0;fz[j]=0;}
            sph_compute(px,py,pz,vx,vy,vz,den,pres,fx,fy,fz,n,1.0,1.0,1000.0,3.0,3.5);
        }

        int N = n < 32 ? 1000000 : (n < 64 ? 100000 : 10000);
        uint64_t start = mach_absolute_time();
        for(int i=0;i<N;i++){
            for(int j=0;j<n;j++){den[j]=0;pres[j]=0;fx[j]=0;fy[j]=0;fz[j]=0;}
            sph_compute(px,py,pz,vx,vy,vz,den,pres,fx,fy,fz,n,1.0,1.0,1000.0,3.0,3.5);
        }
        uint64_t end = mach_absolute_time();
        double ns = (double)(end-start) * info.numer / info.denom;
        printf("n=%d: %.3f ns (%.3f us) per call\n", n, ns/N, ns/N/1000);
        free(px);free(py);free(pz);free(vx);free(vy);free(vz);free(den);free(pres);free(fx);free(fy);free(fz);
    }
    return 0;
}
