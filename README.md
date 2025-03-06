### 项目描述

**Ollama Text Compressor**  
这是一个使用 Go 语言编写的工具，用于将大文本文件分块并通过 Ollama API 进行压缩处理。它支持并发处理和两种 API 端点（`generate` 和 `chat`），能够高效地将文本压缩而不丢失重要信息。项目包含进度条显示、错误重试机制和灵活的配置选项，适用于处理大型文本文件。

---

### README

````markdown
# Ollama Text Compressor

Ollama Text Compressor 是一个命令行工具，用于将大型文本文件分块并通过 Ollama API 进行压缩处理。它利用并发处理和错误重试机制，确保高效且可靠地压缩文本，同时保留重要信息。支持 Ollama 的 `generate` 和 `chat` 端点，用户可以根据需要选择模型和配置参数。

## 功能特性

- **文本分块处理**：将大文件分割成固定大小的块（默认 4000 字节）。
- **并发处理**：支持最多 3 个并发请求（可配置），提高处理效率。
- **错误重试**：对失败的 API 请求自动重试，最多 3 次。
- **进度条显示**：实时显示文件分块和处理进度。
- **灵活配置**：通过命令行参数指定输入文件、输出文件、模型、API 端点和 Ollama URL。
- **压缩统计**：输出压缩比率和文件大小变化。

## 安装

### 前提条件

- [Go](https://golang.org/dl/) 1.16 或更高版本
- 运行中的 [Ollama](https://ollama.ai/) 服务（默认 URL: `http://localhost:11434/api`）
- 支持的 Ollama 模型（例如 `llama3.2-vision:latest`）

### 安装步骤

1. 克隆仓库：
   ```bash
   git clone https://github.com/xxlv/ai-llmtxt.git
   cd ollama-text-compressor
   ```
````

2. 编译项目：
   ```bash
   go build -o ollama-text-compressor
   ```
3. （可选）将可执行文件移动到 PATH 中：
   ```bash
   mv ollama-text-compressor /usr/local/bin/
   ```

## 使用方法

### 基本用法

```bash
./ollama-text-compressor -input <输入文件> -output <输出文件>
```

### 命令行参数

- `-input`（必填）：输入文件路径，例如 `input.txt`。
- `-output`（默认：`llm.txt`）：输出文件路径。
- `-model`（默认：`llama3.2-vision:latest`）：使用的 Ollama 模型名称。
- `-api`（默认：`generate`）：Ollama API 端点，可选 `generate` 或 `chat`。
- `-url`（默认：`http://localhost:11434/api`）：Ollama API 基础 URL。

### 示例

将 `book.txt` 压缩并保存到 `compressed.txt`：

```bash
./ollama-text-compressor -input book.txt -output compressed.txt -model llama3.2-vision:latest -api generate
```

输出示例：

```
Processing file: book.txt (4.39 MB)
Using Ollama model: llama3.2-vision:latest
Using API endpoint: generate
Splitting file into 1151 chunks
Chunking file [==================================================] (1151/1151)
Processing with Ollama [==================================================] (1151/1151)
Compression complete: 1151 of 1151 chunks processed successfully (0 errors)
Output saved to compressed.txt
Compression ratio: 2.34x (from 4.39 MB to 1.88 MB)
```

## 配置常量

可以在代码中修改以下常量以调整行为：

- `chunkSize`：每个分块的大小（默认 4000 字节）。
- `maxConcurrentRequests`：最大并发请求数（默认 3）。
- `requestTimeout`：HTTP 请求超时（默认 120 秒）。
- `maxRetries`：失败重试次数（默认 3）。
- `retryDelay`：重试间隔（默认 2 秒）。

## 注意事项

- 确保 Ollama 服务正在运行，并且指定的模型已拉取。
- 对于 `generate` 端点，工具会处理流式响应；对于 `chat` 端点，期望单次 JSON 响应。
- 如果遇到解析错误，请检查 Ollama API 的响应格式并调整代码。

## 许可证

本项目采用 MIT 许可证。详情见 [LICENSE](LICENSE) 文件。

## 致谢

- [Ollama](https://ollama.ai/)：提供强大的语言模型 API。
- [schollz/progressbar](https://github.com/schollz/progressbar)：提供美观的进度条。

