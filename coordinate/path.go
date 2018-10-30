package coordinate

import (
	"fmt"
	"path/filepath"
)

const (
	DefaultBaseDir             = "/cache-manager"
	DefaultProductDir          = "/products"
	DefaultTopomDir            = "/topom"
	DefaultGroupDir            = "/groups"
	DefaultSentinelDir         = "/sentinel"
	DefaultGSLBDir             = "/gslb"
	DefaultTemplateFileDir     = "/template-files"
)

func ProductDir() string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir))
}

func ProductPath(productName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir, productName))
}

func TopomPath(productName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir, productName, DefaultTopomDir))
}

func GroupDir(productName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir, productName, DefaultGroupDir))
}

func GroupPath(productName, groupName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir, productName, DefaultGroupDir, fmt.Sprintf("group-%s", groupName)))
}

func SentinelPath(productName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultProductDir, productName, DefaultSentinelDir))
}

func GSLBDir() string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultGSLBDir))
}

func GSLBPath(gslbName, productName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultGSLBDir, gslbName, productName))
}

func TemplateFileDir() string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultTemplateFileDir))
}

func TemplateFilePath(fileName string) string {
	return filepath.ToSlash(filepath.Join(DefaultBaseDir, DefaultTemplateFileDir, fileName))
}
