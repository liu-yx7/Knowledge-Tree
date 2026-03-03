// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/plugin/ragflow/crypt.go
package ragflow

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
)

// ==================== RSA 密码加密器 ====================

// Encryptor RSA 密码加密器
// 用于加密发送给 RAGFlow 的密码，与 RAGFlow Python 实现兼容
type Encryptor struct {
	publicKey *rsa.PublicKey
}

// NewEncryptor 创建 RSA 加密器
// 参数: publicKeyPEM - PEM 格式的公钥
// 返回: 加密器实例或错误
func NewEncryptor(publicKeyPEM []byte) (*Encryptor, error) {
	if len(publicKeyPEM) == 0 {
		return nil, fmt.Errorf("公钥不能为空")
	}

	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	return &Encryptor{
		publicKey: publicKey,
	}, nil
}

// EncryptPassword 加密密码
// 加密流程: 明文密码 → Base64 编码 → RSA PKCS1v15 加密 → Base64 编码
// 此流程与 RAGFlow Python 实现 (api/utils/crypt.py) 完全兼容
// 参数: password - 明文密码
// 返回: Base64 编码的加密密码或错误
func (e *Encryptor) EncryptPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("密码不能为空")
	}

	// Step 1: 对密码进行 Base64 编码
	// Python: base64.b64encode(password.encode("utf-8"))
	passwordB64 := base64.StdEncoding.EncodeToString([]byte(password))

	// Step 2: RSA PKCS1_v1_5 加密
	// Python: PKCS1_v1_5.new(rsa_key).encrypt(password_b64)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, e.publicKey, []byte(passwordB64))
	if err != nil {
		return "", fmt.Errorf("RSA 加密失败: %w", err)
	}

	// Step 3: 对加密结果进行 Base64 编码
	// Python: base64.b64encode(encrypted).decode("utf-8")
	result := base64.StdEncoding.EncodeToString(encrypted)

	return result, nil
}

// GetPublicKey 获取公钥（用于测试）
func (e *Encryptor) GetPublicKey() *rsa.PublicKey {
	return e.publicKey
}

// ==================== 公钥解析 ====================

// parsePublicKey 解析 PEM 格式的公钥
// 支持两种格式:
// - PKIX 格式: -----BEGIN PUBLIC KEY-----
// - PKCS1 格式: -----BEGIN RSA PUBLIC KEY-----
func parsePublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("无法解析 PEM 格式，请检查公钥格式是否正确")
	}

	switch block.Type {
	case "PUBLIC KEY":
		// PKIX 格式 (RAGFlow 使用此格式)
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKIX 公钥失败: %w", err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("公钥不是 RSA 类型")
		}
		return rsaPub, nil

	case "RSA PUBLIC KEY":
		// PKCS1 格式
		rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKCS1 公钥失败: %w", err)
		}
		return rsaPub, nil

	default:
		return nil, fmt.Errorf("不支持的公钥类型: %s", block.Type)
	}
}

// ==================== 密码生成 ====================

// 密码字符集：a-z + A-Z + 0-9（不含特殊字符，避免 URL 编码和 JSON 转义问题）
const passwordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// 密码长度：32 字符，提供约 190 bits 熵值
const passwordLength = 32

// GenerateSecurePassword 生成安全的随机密码
// 用于为 Memos 用户自动生成 RAGFlow 账户密码
// 返回: 32 字符的随机密码字符串
func GenerateSecurePassword() string {
	b := make([]byte, passwordLength)
	charsetLen := big.NewInt(int64(len(passwordCharset)))

	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// 在极少数情况下随机数生成失败时，使用备选方案
			// 这不应该发生，但为了健壮性保留
			b[i] = passwordCharset[i%len(passwordCharset)]
			continue
		}
		b[i] = passwordCharset[n.Int64()]
	}

	return string(b)
}

// GenerateRAGFlowCredentials 为 Memos 用户生成 RAGFlow 凭据
// 邮箱格式: {memosUserID}@knowtree.local（简洁、唯一、无特殊字符问题）
// 参数: memosUserID - Memos 用户 ID
// 返回: email（邮箱）, password（随机密码）
func GenerateRAGFlowCredentials(memosUserID int32) (email, password string) {
	email = fmt.Sprintf("%d@knowtree.local", memosUserID)
	password = GenerateSecurePassword()
	return email, password
}
