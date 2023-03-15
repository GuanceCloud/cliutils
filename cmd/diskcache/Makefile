all:
	GOOS=linux GOARCH=amd64 go build -o dist/dc main.go
	GOOS=windows GOARCH=amd64 go build -o dist/dc.exe main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/dc.mac main.go
