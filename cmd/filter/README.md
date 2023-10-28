## Filter 调试器

在编写各种黑名单时，普通用户难以确定规则是否编写正确，这给调试具体的数据策略造成一些麻烦。

通过 filter 调试器，可以得知具体的 filter 规则是否编写正确，对于语法错误的规则，会提示具体的错误位置。

编译：

```bash
$ ./build.sh
```

```bash
# 正确的 filter 规则示例
$ ./fdbg -condition-path filter-sample.txt
Parse 3 conditions ok


# 错误的 filter 规则示例：有错误位置提示
$ ./fdbg -condition-path filter-bad-sample.txt
2023/10/28 10:26:33 GetConds: 1:46 parse error: unterminated quoted string
```
