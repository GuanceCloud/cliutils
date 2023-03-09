# https://golangci-lint.run/usage/install/#local-installation
lint: lint_deps
	rm -rf lint.err
	@echo '============== lint linux/386 ==================='                  | tee -a lint.err
	GOARCH=386 GOOS=linux golangci-lint run --fix --allow-parallel-runners     | tee -a lint.err
	@echo '============== lint windows/386 ==================='                | tee -a lint.err
	GOARCH=386 GOOS=windows golangci-lint run --fix --allow-parallel-runners   | tee -a lint.err
	@echo '============== lint darwin/amd64 ==================='               | tee -a lint.err
	GOARCH=amd64 GOOS=darwin golangci-lint run --fix --allow-parallel-runners  | tee -a lint.err
	@echo '============== lint linux/amd64 ==================='                | tee -a lint.err
	GOARCH=amd64 GOOS=linux golangci-lint run --fix --allow-parallel-runners   | tee -a lint.err
	@echo '============== lint windows/amd64 ==================='              | tee -a lint.err
	GOARCH=amd64 GOOS=windows golangci-lint run --fix --allow-parallel-runners | tee -a lint.err
	@echo '============== lint linux/arm ==================='                  | tee -a lint.err
	GOARCH=arm GOOS=linux golangci-lint run --fix  --allow-parallel-runners    | tee -a lint.err
	@echo '============== lint linux/arm64 ==================='                | tee -a lint.err
	GOARCH=arm64 GOOS=linux golangci-lint run --fix  --allow-parallel-runners  | tee -a lint.err

lint_deps: gofmt vet

vet:
	@go vet ./...

gofmt:
	@GO111MODULE=off gofmt -l $(shell find . -type f -name '*.go'| grep -v "/vendor/\|/.git/")

copyright_check:
	@python3 copyright.py --dry-run && \
		{ echo "copyright check ok"; exit 0; } || \
		{ echo "copyright check failed"; exit -1; }

copyright_check_auto_fix:
	@python3 copyright.py --fix
