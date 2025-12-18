package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	ListenAddr   = "172.16.0.1:80" // 监听本地IP:端口
	DownloadDir  = "/home/download" // 文件存放目录（上传和下载的文件都在这里）
	MaxUploadSize = 1024 * 1024 * 10240 // 最大上传文件大小限制（10240MB）
	UploadPassword = "Rainly520" // 上传密码
)

func main() {
	if err := os.MkdirAll(DownloadDir, 0755); err != nil {
		log.Fatalf("创建目录失败：%v", err)
	}
	// log.Printf("文件存储目录：%s", DownloadDir)

	http.HandleFunc("/", downloadHandler)
	http.HandleFunc("/upload", uploadHandler)

	// 启动 HTTP 服务
	log.Printf("RainlyHTTP 服务已启动，监听：%s", ListenAddr)
	// log.Printf("下载示例：http://172.16.0.1/文件名（例：http://172.16.0.1/1.zip）")
	// log.Printf("上传示例：POST http://172.16.0.1/upload（form-data 格式，字段名 file）")
	if err := http.ListenAndServe(ListenAddr, nil); err != nil {
		log.Fatalf("服务启动失败：%v", err)
	}
}

// 下载服务
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// 只允许 GET 请求
	if r.Method != http.MethodGet {
		http.Error(w, "仅支持 GET 请求（文件下载）", http.StatusMethodNotAllowed)
		return
	}

	// 获取 URL 中的文件名
	filename := strings.TrimPrefix(r.URL.Path, "/")
	if filename == "" {
		http.Error(w, "请指定文件名（例：http://172.16.0.1/1.zip）", http.StatusBadRequest)
		return
	}

	// 安全处理：防止路径穿越攻击
	safePath := filepath.Join(DownloadDir, filename)
	if !strings.HasPrefix(safePath, DownloadDir) {
		http.Error(w, "非法文件路径", http.StatusForbidden)
		return
	}

	// 读取文件并返回给客户端（支持断点续传、大文件下载）
	http.ServeFile(w, r, safePath)
}

// 上传服务
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// 只允许 POST 请求
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 请求（文件上传）", http.StatusMethodNotAllowed)
		return
	}

	var inputPassword string

	// 从请求头获取
	inputPassword = r.Header.Get("X-Upload-Password")
	if inputPassword == "" {
		// 从 form-data 字段获取（兼容表单上传）
		inputPassword = r.FormValue("password")
		if inputPassword == "" {
			// 从 URL 参数获取（备用，如 ?password=xxx）
			inputPassword = r.URL.Query().Get("password")
		}
	}

	// 验证密码
	if inputPassword != UploadPassword {
		http.Error(w, "密码错误：未授权上传", http.StatusUnauthorized)
		log.Printf("上传失败：密码错误（IP：%s，输入密码：%s）", r.RemoteAddr, inputPassword)
		return
	}

	// 限制请求体大小（防止超大文件上传）
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		http.Error(w, "上传失败：文件过大", http.StatusRequestEntityTooLarge)
		return
	}

	// 获取上传的文件（form-data 中的字段名是 "file"）
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "获取上传文件失败："+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close() // 确保文件句柄关闭

	// 安全处理文件名：
	// - 去除路径分隔符（防止路径穿越）
	// - 去除前后空格
	filename := strings.TrimSpace(fileHeader.Filename)
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	if filename == "" {
		http.Error(w, "文件名不能为空", http.StatusBadRequest)
		return
	}

	// 构建安全的文件路径
	safePath := filepath.Join(DownloadDir, filename)
	// 再次校验路径（双重保险）
	if !strings.HasPrefix(safePath, DownloadDir) {
		http.Error(w, "非法文件名", http.StatusForbidden)
		return
	}

	// 创建文件并写入上传内容
	dstFile, err := os.Create(safePath)
	if err != nil {
		http.Error(w, "创建文件失败："+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dstFile.Close()

	// 复制上传文件内容到本地文件
	_, err = io.Copy(dstFile, file)
	if err != nil {
		http.Error(w, "文件上传失败："+err.Error(), http.StatusInternalServerError)
		return
	}

	// 上传成功响应
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	response := `{"code":200,"message":"文件上传成功","filename":"` + filename + `","savePath":"` + safePath + `"}`
	_, _ = w.Write([]byte(response))
	// log.Printf("文件上传成功：%s（大小：%d bytes，上传IP：%s）", filename, fileHeader.Size, r.RemoteAddr)
}