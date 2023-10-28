## Filter 调试器

通过 filter 调试器，可以得知具体的 filter 规则是否编写正确。

编译：

```bash
$ ./build.sh
```

```bash
# 正确的 filter 规则示例
$ ./fdbg -condition-path filter-sample.txt
Parse 3 conditions ok


# 错误的 filter 规则示例
$ ./fdbg -condition-path filter-bad-sample.txt
2023/10/28 10:26:33 GetConds: 1:46 parse error: unterminated quoted string
```
