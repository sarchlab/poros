// Freestanding Linux AArch64 fixtures: no libc, emulator hooks or host helpers.
typedef unsigned long u64;
typedef long i64;
extern void _start(void);
static i64 call(u64 nr,u64 a,u64 b,u64 c) {
 register u64 x0 asm("x0")=a,x1 asm("x1")=b,x2 asm("x2")=c,x8 asm("x8")=nr;
 asm volatile("svc #0":"+r"(x0):"r"(x1),"r"(x2),"r"(x8):"memory");return (i64)x0;
}
static u64 length(const char *s){u64 n=0;while(s[n])n++;return n;}
static void print(const char*s){call(64,1,(u64)s,length(s));}
static int equal(const char*a,const char*b){while(*a&&*a==*b){a++;b++;}return *a==*b;}
#if CASE == 3
static volatile u64 bss[32];
static volatile u64 data=0x123456789abcdef0UL;
#endif
#if CASE == 4
static volatile double a=1.5,b=2.0,c=0.25,one=1.0,ten=10.0;
static volatile union {u64 bits;double value;} edge_a={0x3ff0000002000000UL},edge_b={0x3feffffffc000000UL};
#endif
int guest_main(u64 *sp) {
#if CASE == 1
 print("hello, ELF\n");call(64,2,(u64)"stderr\n",7);return 0;
#elif CASE == 2
 if(((u64)sp&15)||sp[0]!=3)return 10;
 char **argv=(char**)&sp[1];
 if(!equal(argv[0],"fixture")||!equal(argv[1],"alpha")||!equal(argv[2],"beta")||argv[3])return 11;
 char **env=argv+4;if(!env[0]||!equal(env[0],"MODE=test")||env[1])return 12;
 u64 *aux=(u64*)(env+2),seen=0;
 for(;aux[0];aux+=2){
  if(aux[0]==3){if(!aux[1])return 13; seen|=1; volatile u64 h=*(u64*)aux[1];if(!h)return 14;}
  if(aux[0]==4){if(aux[1]!=56)return 15;seen|=2;}
  if(aux[0]==5){if(aux[1]!=4)return 16;seen|=4;}
  if(aux[0]==6){if(aux[1]<4096||(aux[1]&(aux[1]-1)))return 17;seen|=8;}
  if(aux[0]==9){if(aux[1]!=(u64)&_start)return 18;seen|=16;}
  if(aux[0]==25){volatile char random=*(char*)aux[1];(void)random;seen|=32;}
 }
 if(seen!=63)return 19;
 print(argv[1]);print(" ");print(argv[2]);print(" ");print(env[0]);print("\n");return 0;
#elif CASE == 3
 for(u64 i=0;i<32;i++){if(bss[i])return 20;bss[i]=i*17;}
 if(data!=0x123456789abcdef0UL)return 21;
 u64 sum=0;for(u64 i=0;i<32;i++)sum+=bss[i];
 call(64,1,(u64)&sum,8);return 0;
#elif CASE == 4
 // Independently checked outputs: 7*13+5=96; 1.5*2+0.25=3.25;
 // (1.5+0.25)/2=0.875. Values are deliberately exactly representable.
 volatile u64 x=7,y=13;u64 integer=x*y+5;
 double av=a,bv=b,cv=c,fused,ratio=(av+cv)/bv;
 asm("fmadd %d0,%d1,%d2,%d3":"=w"(fused):"w"(av),"w"(bv),"w"(cv));
 double ea=edge_a.value,eb=edge_b.value,minus_one=-1.0,edge,inexact=one/ten;
 asm("fmadd %d0,%d1,%d2,%d3":"=w"(edge):"w"(ea),"w"(eb),"w"(minus_one));
 call(64,1,(u64)&integer,8);call(64,1,(u64)&fused,8);call(64,1,(u64)&ratio,8);
 call(64,1,(u64)&edge,8);call(64,1,(u64)&inexact,8);return 0;
#elif CASE == 5
 i64 values[4];values[0]=call(64,1,0,0);values[1]=call(64,99,0,0);values[2]=call(64,1,0,1);
 values[3]=call(64,0x100000001UL,(u64)"X",1);call(64,1,(u64)values,sizeof(values));return 0;
#elif CASE == 6
 return *(volatile int*)0x1000;
#elif CASE == 7
 *(volatile int*)&_start=0;return 0;
#elif CASE == 8
 u64 code=0xd65f03c0;((void(*)(void))&code)();return 0;
#elif CASE == 9
 call(172,0,0,0);return 0;
#elif CASE == 10
 asm volatile(".inst 0xffffffff");return 0;
#elif CASE == 11
 for(;;)asm volatile("nop");
#elif CASE == 12
 print("output limit\n");return 0;
#elif CASE == 13
 call(94,0x123,0,0);return 99;
#elif CASE == 14
 if(sp[0]!=2)return 30;
 char *s=(char*)sp[2];u64 n=0;while(*s){if(*s<'0'||*s>'9')return 31;n=n*10+*s++-'0';}
 if(n>1000000)return 32;
 volatile u64 sum=0;for(u64 i=0;i<n;i++)sum+=i;
 call(64,1,(u64)&sum,8);return 0;
#endif
}
