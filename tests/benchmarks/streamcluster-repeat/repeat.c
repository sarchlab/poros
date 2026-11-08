typedef unsigned long u64;
extern int guest_main(u64*);
int repeat_main(u64 *sp){
 u64 argc=sp[0]; if(argc!=4) return 90;
 const char *s=(const char*)sp[argc]; u64 k=0;
 if(!*s) return 91;
 while(*s){if(*s<'0'||*s>'9') return 91;k=k*10+(*s++-'0');if(k>64)return 92;}
 if(!k)return 93;
 sp[0]=argc-1;
 for(u64 i=0;i<k;i++){int r=guest_main(sp);if(r)return r;}
 return 0;
}
