package ragflow_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== 测试用公钥 ====================

// RAGFlow 官方公钥（从 ragflow/conf/public.pem 拷贝）
const testPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArq9XTUSeYr2+N1h3Afl/
z8Dse/2yD0ZGrKwx+EEEcdsBLca9Ynmx3nIB5obmLlSfmskLpBo0UACBmB5rEjBp
2Q2f3AG3Hjd4B+gNCG6BDaawuDlgANIhGnaTLrIqWrrcm4EMzJOnAOI1fgzJRsOO
UEfaS318Eq9OVO3apEyCCt0lOQK6PuksduOjVxtltDav+guVAA068NrPYmRNabVK
RNLJpL8w4D44sfth5RvZ3q9t+6RTArpEtc5sh5ChzvqPOzKGMXW83C95TxmXqpbK
6olN4RevSfVjEAgCydH6HN6OhtOQEcnrU97r9H0iZOWwbw3pVrZiUkuRD1R56Wzs
2wIDAQAB
-----END PUBLIC KEY-----`

// PKCS1 格式公钥（用于测试格式兼容性）
const testPKCS1PublicKeyPEM = `-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEArq9XTUSeYr2+N1h3Afl/z8Dse/2yD0ZGrKwx+EEEcdsBLca9Ynmx
3nIB5obmLlSfmskLpBo0UACBmB5rEjBp2Q2f3AG3Hjd4B+gNCG6BDaawuDlgANIh
GnaTLrIqWrrcm4EMzJOnAOI1fgzJRsOOUEfaS318Eq9OVO3apEyCCt0lOQK6Puks
duOjVxtltDav+guVAA068NrPYmRNabVKRNLJpL8w4D44sfth5RvZ3q9t+6RTArpE
tc5sh5ChzvqPOzKGMXW83C95TxmXqpbK6olN4RevSfVjEAgCydH6HN6OhtOQEcnr
U97r9H0iZOWwbw3pVrZiUkuRD1R56Wzs2wIDAQAB
-----END RSA PUBLIC KEY-----`

// ==================== NewEncryptor 测试 ====================

func TestNewEncryptor_ValidPKIXKey(t *testing.T) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}
	if enc == nil {
		t.Fatal("Encryptor 为空")
	}

	// 通过加密操作间接验证公钥已正确加载
	encrypted, err := enc.EncryptPassword("test")
	if err != nil {
		t.Fatalf("加密失败，说明公钥未正确加载: %v", err)
	}
	// 对于 2048-bit RSA 密钥，Base64 编码后长度约 344 字符
	if len(encrypted) < 300 || len(encrypted) > 400 {
		t.Errorf("加密结果长度不正确: %d（说明公钥位数可能有误）", len(encrypted))
	}
}

func TestNewEncryptor_EmptyKey(t *testing.T) {
	_, err := ragflow.NewEncryptor([]byte{})
	if err == nil {
		t.Fatal("空公钥应该返回错误")
	}
	if !strings.Contains(err.Error(), "公钥不能为空") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

func TestNewEncryptor_InvalidPEM(t *testing.T) {
	invalidPEM := []byte("this is not a valid PEM")
	_, err := ragflow.NewEncryptor(invalidPEM)
	if err == nil {
		t.Fatal("无效 PEM 应该返回错误")
	}
	if !strings.Contains(err.Error(), "无法解析 PEM 格式") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

func TestNewEncryptor_UnsupportedKeyType(t *testing.T) {
	ecKeyPEM := `-----BEGIN EC PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEYD54V/vp+54P9DXarYqx4MPcm+HK
RIQzNasYSoRQHQ/6S6Ps8tpMcT+KvIIC8W/e9k0W7Cm72M1P9jU7SLf/vg==
-----END EC PUBLIC KEY-----`
	_, err := ragflow.NewEncryptor([]byte(ecKeyPEM))
	if err == nil {
		t.Fatal("EC 公钥应该返回错误")
	}
}

// ==================== EncryptPassword 测试 ====================

func TestEncryptPassword_BasicPassword(t *testing.T) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}

	password := "MyPassword123"
	encrypted, err := enc.EncryptPassword(password)
	if err != nil {
		t.Fatalf("加密密码失败: %v", err)
	}

	_, err = base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Errorf("加密结果不是有效的 Base64: %v", err)
	}

	if len(encrypted) < 300 || len(encrypted) > 400 {
		t.Errorf("加密结果长度不正确: %d", len(encrypted))
	}
}

func TestEncryptPassword_EmptyPassword(t *testing.T) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}

	_, err = enc.EncryptPassword("")
	if err == nil {
		t.Fatal("空密码应该返回错误")
	}
	if !strings.Contains(err.Error(), "密码不能为空") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

func TestEncryptPassword_SpecialCharacters(t *testing.T) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}

	testCases := []string{
		"password!@#$%^&*()",
		"密码测试123",
		"пароль123",
		"パスワード",
		"emoji😀test",
		"spaces in password",
		"tab\tand\nnewline",
	}

	for _, password := range testCases {
		t.Run(password, func(t *testing.T) {
			encrypted, err := enc.EncryptPassword(password)
			if err != nil {
				t.Errorf("加密密码 %q 失败: %v", password, err)
				return
			}

			_, err = base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Errorf("加密结果不是有效的 Base64: %v", err)
			}
		})
	}
}

func TestEncryptPassword_DifferentResultsEachTime(t *testing.T) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}

	password := "SamePassword"
	encrypted1, _ := enc.EncryptPassword(password)
	encrypted2, _ := enc.EncryptPassword(password)

	if encrypted1 == encrypted2 {
		t.Error("两次加密结果相同，可能随机填充有问题")
	}
}

// ==================== 加密解密验证测试 ====================

func TestEncryptPassword_CanBeDecrypted(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("导出公钥失败: %v", err)
	}
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	enc, err := ragflow.NewEncryptor(pubKeyPEM)
	if err != nil {
		t.Fatalf("创建 Encryptor 失败: %v", err)
	}

	password := "TestPassword123"
	encrypted, err := enc.EncryptPassword(password)
	if err != nil {
		t.Fatalf("加密密码失败: %v", err)
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("Base64 解码失败: %v", err)
	}

	decryptedB64, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedBytes)
	if err != nil {
		t.Fatalf("RSA 解密失败: %v", err)
	}

	decrypted, err := base64.StdEncoding.DecodeString(string(decryptedB64))
	if err != nil {
		t.Fatalf("内层 Base64 解码失败: %v", err)
	}

	if string(decrypted) != password {
		t.Errorf("解密结果不匹配: got %q, want %q", string(decrypted), password)
	}
}

// ==================== GenerateSecurePassword 测试 ====================

func TestGenerateSecurePassword_Length(t *testing.T) {
	password := ragflow.GenerateSecurePassword()
	if len(password) != 32 {
		t.Errorf("密码长度不正确: got %d, want 32", len(password))
	}
}

func TestGenerateSecurePassword_Charset(t *testing.T) {
	password := ragflow.GenerateSecurePassword()
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	for _, c := range password {
		if !strings.ContainsRune(validChars, c) {
			t.Errorf("密码包含无效字符: %c", c)
		}
	}
}

func TestGenerateSecurePassword_Uniqueness(t *testing.T) {
	passwords := make(map[string]bool)
	for i := 0; i < 100; i++ {
		p := ragflow.GenerateSecurePassword()
		if passwords[p] {
			t.Errorf("生成了重复密码: %s", p)
		}
		passwords[p] = true
	}
}

func TestGenerateSecurePassword_Distribution(t *testing.T) {
	charCount := make(map[rune]int)
	for i := 0; i < 1000; i++ {
		p := ragflow.GenerateSecurePassword()
		for _, c := range p {
			charCount[c]++
		}
	}

	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, c := range validChars {
		if charCount[c] == 0 {
			t.Errorf("字符 %c 从未出现在生成的密码中", c)
		}
	}
}

// ==================== GenerateRAGFlowCredentials 测试 ====================

func TestGenerateRAGFlowCredentials_EmailFormat(t *testing.T) {
	testCases := []struct {
		userID        int32
		expectedEmail string
	}{
		{1, "1@knowtree.local"},
		{42, "42@knowtree.local"},
		{1001, "1001@knowtree.local"},
		{999999, "999999@knowtree.local"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedEmail, func(t *testing.T) {
			email, password := ragflow.GenerateRAGFlowCredentials(tc.userID)

			if email != tc.expectedEmail {
				t.Errorf("邮箱格式不正确: got %q, want %q", email, tc.expectedEmail)
			}

			if len(password) != 32 {
				t.Errorf("密码长度不正确: got %d, want 32", len(password))
			}
		})
	}
}

func TestGenerateRAGFlowCredentials_UniquePasswords(t *testing.T) {
	_, password1 := ragflow.GenerateRAGFlowCredentials(1)
	_, password2 := ragflow.GenerateRAGFlowCredentials(1)

	if password1 == password2 {
		t.Error("同一用户两次生成的密码相同")
	}
}

// ==================== Config.LoadPublicKey 测试 ====================

func TestConfig_LoadPublicKey_FromEnvVar(t *testing.T) {
	t.Setenv("RAGFLOW_PUBLIC_KEY", testPublicKeyPEM)

	cfg := &ragflow.Config{}
	pubKey, err := cfg.LoadPublicKey()
	if err != nil {
		t.Fatalf("从环境变量加载公钥失败: %v", err)
	}

	if string(pubKey) != testPublicKeyPEM {
		t.Error("公钥内容不匹配")
	}
}

func TestConfig_LoadPublicKey_FromConfigPath(t *testing.T) {
	cfg := &ragflow.Config{
		PublicKeyPath: "../../conf/ragflow_public.pem",
	}

	pubKey, err := cfg.LoadPublicKey()
	if err != nil {
		t.Skipf("跳过文件加载测试（公钥文件可能不存在）: %v", err)
	}

	if len(pubKey) == 0 {
		t.Error("公钥内容为空")
	}
}

func TestConfig_LoadPublicKey_FileNotExist(t *testing.T) {
	cfg := &ragflow.Config{
		PublicKeyPath: "/nonexistent/path/public.pem",
	}

	_, err := cfg.LoadPublicKey()
	if err == nil {
		t.Fatal("不存在的文件应该返回错误")
	}
	if !strings.Contains(err.Error(), "公钥文件不存在") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

// ==================== 基准测试 ====================

func BenchmarkEncryptPassword(b *testing.B) {
	enc, err := ragflow.NewEncryptor([]byte(testPublicKeyPEM))
	if err != nil {
		b.Fatalf("创建 Encryptor 失败: %v", err)
	}

	password := "BenchmarkPassword123"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = enc.EncryptPassword(password)
	}
}

func BenchmarkGenerateSecurePassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ragflow.GenerateSecurePassword()
	}
}
