package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// uploadHandler 处理文件上传
func uploadHandler(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "获取上传文件失败: " + err.Error(),
		})
		return
	}

	// 确保文件是ZIP格式
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "仅支持ZIP格式的文件",
		})
		return
	}

	// 创建上传目录
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	// 保存上传的文件
	filePath := filepath.Join(uploadDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "保存文件失败: " + err.Error(),
		})
		return
	}

	// 解压ZIP文件
	extractPath := filepath.Join(uploadDir, strings.TrimSuffix(file.Filename, ".zip"))
	if err := extractZip(filePath, extractPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "解压文件失败: " + err.Error(),
		})
		return
	}

	// 读取startScript参数
	startScript := c.PostForm("startScript")

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     fmt.Sprintf("文件已成功上传并解压到: %s", extractPath),
		"extractPath": extractPath,
		"startScript": startScript,
	})
}

// extractZip 解压ZIP文件到指定目录
func extractZip(zipPath, extractPath string) error {
	// 创建解压目录
	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return err
	}

	// 打开ZIP文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	// 遍历ZIP中的文件
	for _, file := range zipReader.File {
		// 构建文件路径
		filePath := filepath.Join(extractPath, file.Name)

		// 检查路径安全性（防止路径遍历攻击）
		if !strings.HasPrefix(filePath, extractPath+string(os.PathSeparator)) {
			return fmt.Errorf("非法文件路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// 是目录，创建目录
			if err := os.MkdirAll(filePath, file.Mode()); err != nil {
				return err
			}
		} else {
			// 是文件，创建目录并写入文件
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return err
			}

			// 打开源文件
			srcFile, err := file.Open()
			if err != nil {
				return err
			}
			defer srcFile.Close()

			// 创建目标文件
			dstFile, err := os.Create(filePath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			// 复制文件内容
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		}
	}

	return nil
}