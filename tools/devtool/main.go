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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultPrivateKeyPath = "configs/auth-private.local.pem"

const usage = "devtool <setup|keygen|public-key|build-time|dirty|binary-path|pgo-binary-path|pgo-profile-exists>"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("用法: " + usage)
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
		publicPath := flags.String("public-output", "", "可选的公钥输出路径")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("解析 keygen 参数: %w", err)
		}
		if err := generateKeyPair(*path, *publicPath); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "已生成 Ed25519 私钥: %s\n", filepath.ToSlash(*path))
		if err == nil && *publicPath != "" {
			_, err = fmt.Fprintf(output, "已生成 Ed25519 公钥: %s\n", filepath.ToSlash(*publicPath))
		}
		return err
	case "public-key":
		flags := flag.NewFlagSet("public-key", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		privatePath := flags.String("private", defaultPrivateKeyPath, "私钥文件路径")
		publicPath := flags.String("output", "", "公钥输出路径")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("解析 public-key 参数: %w", err)
		}
		if *publicPath == "" {
			return errors.New("public-key 必须提供 --output")
		}
		if err := exportPublicKey(*privatePath, *publicPath); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "已导出 Ed25519 公钥: %s\n", filepath.ToSlash(*publicPath))
		return err
	case "build-time":
		_, err := fmt.Fprintln(output, time.Now().UTC().Format(time.RFC3339))
		return err
	case "dirty":
		command := exec.Command("git", "status", "--porcelain")
		value, err := command.Output()
		if err != nil {
			return fmt.Errorf("检查 Git 工作区: %w", err)
		}
		_, err = fmt.Fprintln(output, strings.TrimSpace(string(value)) != "")
		return err
	case "binary-path":
		path := "bin/goba"
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		_, err := fmt.Fprintln(output, path)
		return err
	case "pgo-binary-path":
		path := "bin/goba-pgo"
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		_, err := fmt.Fprintln(output, path)
		return err
	case "pgo-profile-exists":
		info, err := os.Stat("default.pgo")
		if err != nil {
			return err
		}
		if info.IsDir() {
			return errors.New("default.pgo 不是文件")
		}
		return nil
	default:
		return fmt.Errorf("未知命令 %q，用法: %s", args[0], usage)
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
		if err := generateKeyPair(keyPath, ""); err != nil {
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
	_, err = fmt.Fprintln(output, "本地文件已准备完成。使用外部依赖时运行 task db:migrate 和 task run；完整环境运行 task compose:up。")
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
	return generateKeyPair(path, "")
}

func generateKeyPair(privatePath, publicPath string) error {
	if err := os.MkdirAll(filepath.Dir(privatePath), 0o700); err != nil {
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
	privateBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	created, err := writeExclusive(privatePath, privateBlock, 0o600)
	if err != nil {
		return fmt.Errorf("创建私钥文件: %w", err)
	}
	if !created {
		return fmt.Errorf("私钥文件已存在，拒绝覆盖: %s", filepath.ToSlash(privatePath))
	}
	if publicPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(publicPath), 0o750); err != nil {
		_ = os.Remove(privatePath)
		return fmt.Errorf("创建公钥目录: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		_ = os.Remove(privatePath)
		return fmt.Errorf("编码 Ed25519 公钥: %w", err)
	}
	publicBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	publicCreated, err := writeExclusive(publicPath, publicBlock, 0o644)
	if err != nil || !publicCreated {
		_ = os.Remove(privatePath)
		if err != nil {
			return fmt.Errorf("创建公钥文件: %w", err)
		}
		return fmt.Errorf("公钥文件已存在，拒绝覆盖: %s", filepath.ToSlash(publicPath))
	}
	return nil
}

func exportPublicKey(privatePath, publicPath string) error {
	// #nosec G304 -- 私钥路径由开发者通过显式命令参数提供。
	content, err := os.ReadFile(privatePath)
	if err != nil {
		return fmt.Errorf("读取 Ed25519 私钥: %w", err)
	}
	block, rest := pem.Decode(content)
	if block == nil || len(rest) != 0 {
		return errors.New("私钥必须是 Ed25519 PKCS#8 PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return errors.New("私钥必须是 Ed25519 PKCS#8 PEM")
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return errors.New("私钥不是 Ed25519 私钥")
	}
	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return fmt.Errorf("编码 Ed25519 公钥: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(publicPath), 0o750); err != nil {
		return fmt.Errorf("创建公钥目录: %w", err)
	}
	created, err := writeExclusive(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o644)
	if err != nil {
		return fmt.Errorf("创建公钥文件: %w", err)
	}
	if !created {
		return fmt.Errorf("公钥文件已存在，拒绝覆盖: %s", filepath.ToSlash(publicPath))
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
