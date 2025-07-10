BUILD = ./build
NAME = tmusic

build: clean
	@mkdir -p $(BUILD)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD)/$(NAME)_linux_amd64 main.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD)/$(NAME)_linux_arm64 main.go
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD)/$(NAME)_windows_amd64 main.go
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD)/$(NAME)_darwin_arm64 main.go
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD)/$(NAME)_darwin_amd64 main.go
	@upx $(BUILD)/$(NAME)_linux_amd64
	@upx $(BUILD)/$(NAME)_linux_arm64
	@upx $(BUILD)/$(NAME)_windows_amd64

clean:
	@rm -rf $(BUILD)
