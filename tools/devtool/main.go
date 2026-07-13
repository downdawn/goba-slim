package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const defaultPrivateKeyPath = "configs/auth-private.local.pem"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("用法: devtool <setup|keygen>")
	}
	switch args[0] {
	case "setup":
		flags := flag.NewFlagSet("setup", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		root := flags.String("root", ".", "项目根目录")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("解析 setup 参数: %w", err)
		}
		return setup(*root, output)
	case "keygen":
		flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("output", defaultPrivateKeyPath, "私钥输出路径")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("解析 keygen 参数: %w", err)
		}
		if err := generatePrivateKey(*path); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "已生成 Ed25519 私钥: %s\n", filepath.ToSlash(*path))
		return err
	default:
		return fmt.Errorf("未知命令 %q，用法: devtool <setup|keygen>", args[0])
	}
}

func setup(root string, output io.Writer) error {
	configPath := filepath.Join(root, "configs", "config.local.yaml")
	configCreated, err := copyIfMissing(filepath.Join(root, "configs", "config.example.yaml"), configPath, 0o600)
	if err != nil {
		return fmt.Errorf("创建本地 YAML: %w", err)
	}

	keyPath := filepath.Join(root, filepath.FromSlash(defaultPrivateKeyPath))
	keyCreated := false
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		if err := generatePrivateKey(keyPath); err != nil {
			return err
		}
		keyCreated = true
	} else if err != nil {
		return fmt.Errorf("检查私钥文件: %w", err)
	}

	envCreated, err := createLocalEnv(filepath.Join(root, ".env"))
	if err != nil {
		return err
	}

	writeSetupStatus(output, configPath, configCreated)
	writeSetupStatus(output, keyPath, keyCreated)
	writeSetupStatus(output, filepath.Join(root, ".env"), envCreated)
	_, err = fmt.Fprintln(output, "本地文件已准备完成。下一步运行 task dev:init，然后运行 task run。")
	return err
}

func writeSetupStatus(output io.Writer, path string, created bool) {
	status := "保留"
	if created {
		status = "创建"
	}
	_, _ = fmt.Fprintf(output, "%s: %s\n", status, filepath.ToSlash(path))
}

func copyIfMissing(source, target string, mode os.FileMode) (bool, error) {
	// #nosec G304 -- 路径由仓库内固定的 setup 文件清单构造。
	content, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	return writeExclusive(target, content, mode)
}

func createLocalEnv(path string) (bool, error) {
	databasePassword, err := randomHex(32)
	if err != nil {
		return false, fmt.Errorf("生成数据库密码: %w", err)
	}
	redisPassword, err := randomHex(32)
	if err != nil {
		return false, fmt.Errorf("生成 Redis 密码: %w", err)
	}
	content := fmt.Sprintf(`GOBA_APP_ENVIRONMENT=development
GOBA_SERVER_HOST=0.0.0.0
GOBA_SERVER_PORT=8000
GOBA_DATABASE_PASSWORD=%s
GOBA_REDIS_PASSWORD=%s
GOBA_AUTH_PRIVATE_KEY_FILE=%s
`, databasePassword, redisPassword, defaultPrivateKeyPath)
	created, err := writeExclusive(path, []byte(content), 0o600)
	if err != nil {
		return false, fmt.Errorf("创建 .env: %w", err)
	}
	return created, nil
}

func generatePrivateKey(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建私钥目录: %w", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("生成 Ed25519 私钥: %w", err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("编码 Ed25519 私钥: %w", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	created, err := writeExclusive(path, block, 0o600)
	if err != nil {
		return fmt.Errorf("创建私钥文件: %w", err)
	}
	if !created {
		return fmt.Errorf("私钥文件已存在，拒绝覆盖: %s", filepath.ToSlash(path))
	}
	return nil
}

func writeExclusive(path string, content []byte, mode os.FileMode) (bool, error) {
	// #nosec G304 -- setup 使用固定仓库路径，keygen 路径由开发者显式选择；O_EXCL 拒绝覆盖。
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return false, err
	}
	return true, nil
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
