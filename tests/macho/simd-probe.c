#include <stdint.h>
#include <stdio.h>
static void hex(const uint8_t *v){putchar('"');for(unsigned i=0;i<16;i++)printf("%02x",v[i]);putchar('"');}
int main(void) {
 uint8_t a[16],b[16],got_or[16],got_xor[16],got_hi[16], got_cnt[16];
 unsigned minimum, sum, tests=0;
 puts("{\"vectors\":[");
 for(unsigned trial=0;trial<256;trial++){
  for(unsigned i=0;i<16;i++){a[i]=(trial*37+i*17)&255;b[i]=(trial*71+i*13)&255;}
  __asm__ volatile(
   "ldr q0,[%[a]]\n\tldr q1,[%[b]]\n\t"
   "orr v2.16b,v0.16b,v1.16b\n\tstr q2,[%[o]]\n\t"
   "eor v3.16b,v0.16b,v1.16b\n\tstr q3,[%[x]]\n\t"
   "cmhi v4.16b,v0.16b,v1.16b\n\tstr q4,[%[h]]\n\t"
   "cnt v6.8b,v0.8b\n\tstr q6,[%[c]]\n\tuaddlv h7,v0.8b\n\tumov %w[s],v7.h[0]\n\t"
   "uminv b5,v0.16b\n\tumov %w[m],v5.b[0]"
   : [m]"=r"(minimum), [s]"=&r"(sum) : [a]"r"(a),[b]"r"(b),[o]"r"(got_or),[x]"r"(got_xor),[h]"r"(got_hi),[c]"r"(got_cnt)
   : "v0","v1","v2","v3","v4","v5","v6","v7","memory");
  unsigned wantmin=255;
  for(unsigned i=0;i<16;i++){if(a[i]<wantmin)wantmin=a[i];if(got_or[i]!=(uint8_t)(a[i]|b[i])||got_xor[i]!=(uint8_t)(a[i]^b[i])||got_hi[i]!=(a[i]>b[i]?255:0))return 1;tests+=3;}
  if(minimum!=wantmin)return 2;tests++;
  unsigned wantsum=0;
  for(unsigned i=0;i<16;i++) {
   unsigned count=0; for(unsigned bit=0;bit<8;bit++) count+=(a[i]>>bit)&1;
   if(got_cnt[i]!=(i<8?count:0))return 3;
   if(i<8)wantsum+=a[i];tests++;
  }
  if(sum!=wantsum)return 4;tests++;
  if(trial)putchar(',');
  printf("{\"a\":");hex(a);printf(",\"b\":");hex(b);
  printf(",\"or\":");hex(got_or);printf(",\"xor\":");hex(got_xor);
  printf(",\"hi\":");hex(got_hi);printf(",\"cnt\":");hex(got_cnt);
  printf(",\"min\":%u,\"sum\":%u}",minimum,sum);
 }
 printf("],\"cases\":%u,\"all_native_outputs_match_scalar_oracles\":true}\n",tests);
 return 0;
}
