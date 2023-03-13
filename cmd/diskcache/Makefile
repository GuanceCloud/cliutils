all:
	GOOS=linux GOARCH=amd64 go build -o dc main.go
	GOOS=windows GOARCH=amd64 go build -o dc.exe main.go
	GOOS=darwin GOARCH=arm64 go build -o dc.mac main.go
