#ifndef POROS_MACHO_STAT_H
#define POROS_MACHO_STAT_H
#include <sys/stat.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
// stat64's defined ARM64 fields only; capture rejects nonzero padding/reserved
// bytes instead of pretending they were observed as zero.
static int print_file_stat(const struct stat *st) {
 _Static_assert(sizeof(struct stat)==144&&offsetof(struct stat,st_atimespec)==32&&offsetof(struct stat,st_size)==96&&offsetof(struct stat,st_lspare)==124,"Darwin stat64 ABI");
 const unsigned char *raw=(const unsigned char *)st;
 for(unsigned i=28;i<32;i++)if(raw[i])return 1;
 for(unsigned i=124;i<144;i++)if(raw[i])return 2;
 printf("{\"device\":%u,\"mode\":%u,\"links\":%u,\"inode\":\"%llu\",\"uid\":%u,\"gid\":%u,\"rdev\":%u,",(uint32_t)st->st_dev,st->st_mode,st->st_nlink,(unsigned long long)st->st_ino,st->st_uid,st->st_gid,(uint32_t)st->st_rdev);
 const char *names[]={"atime","mtime","ctime","birthtime"};
 const struct timespec *times[]={&st->st_atimespec,&st->st_mtimespec,&st->st_ctimespec,&st->st_birthtimespec};
 for(unsigned i=0;i<4;i++)printf("\"%s\":{\"seconds\":\"%lld\",\"nanoseconds\":\"%lld\"},",names[i],(long long)times[i]->tv_sec,(long long)times[i]->tv_nsec);
 printf("\"size\":\"%lld\",\"blocks\":\"%lld\",\"block_size\":%d,\"flags\":%u,\"generation\":%u}",(long long)st->st_size,(long long)st->st_blocks,st->st_blksize,st->st_flags,st->st_gen);return 0;
}
#endif
