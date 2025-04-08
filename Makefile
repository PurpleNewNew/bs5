# 设置变量
PROJECT_NAME := bs5
GO := go
GO_BUILD := $(GO) build
GO_TEST := $(GO) test
GO_INSTALL := $(GO) install
BIN_DIR := bin
SRC_DIR := cmd

# 默认目标
all: test build

# 编译项目
build:
	@echo "Building project..."
	@echo "BIN_DIR: $(BIN_DIR)"
	@if not exist $(BIN_DIR) (mkdir $(BIN_DIR))
	@$(GO_BUILD) -o $(BIN_DIR)/$(PROJECT_NAME).exe $(SRC_DIR)/main.go

# 运行项目
run:
	@echo "Running project..."
	@$(GO) run $(SRC_DIR)/main.go

# 测试项目
test:
	@echo "Running tests..."
	@$(GO_TEST) -v ./...

# 清理生成的文件
clean:
	@echo "Cleaning up..."
	@if exist $(BIN_DIR) (rmdir /s /q $(BIN_DIR))

.PHONY: all build run test clean