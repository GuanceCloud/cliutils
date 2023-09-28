# Contributing to user_agent

## Check that your changes do not break anything

You can safely run tests and the linting utilities with the default `make` target:

```
$ make
```

Note that this target assumes that you have
[golangci-lint](https://github.com/golangci/golangci-lint) and
[git-valitation](https://github.com/vbatts/git-validation) already installed. If
that is not the case, first install them to get a proper execution of the
default `make` target.

Otherwise, if you want to be more specific, refer to the `Makefile` to check
which target fits your needs. That being said, the default target is usually
what you want.

## Issue reporting

I'm using [Github](https://github.com/mssola/user_agent) in order to host the
code. Thus, in order to report issues you can do it on its [issue
tracker](https://github.com/mssola/user_agent/issues). A couple of notes on
reports:

- Check that the issue has not already been reported or fixed in `master`.
- Try to be concise and precise in your description of the problem.
- Provide a step by step guide on how to reproduce this problem.
- Provide the version you are using (the commit SHA, if possible).

## Pull requests

- Write a [good commit message](https://chris.beams.io/posts/git-commit/).
- Make sure that tests are passing on your local machine (it will also be
checked by the CI system whenever you submit the pull request).
- Update the [changelog](./CHANGELOG.md).
- Try to use the same coding conventions as used in this project.
- Open a pull request with *only* one subject and a clear title and
description. Refrain from submitting pull requests with tons of different
unrelated commits.
