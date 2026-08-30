//go:build darwin
// +build darwin

package main

// macOS 更新说明：
// 客户端通过 .app bundle（zip 资产）整体替换完成更新，而非替换单个可执行文件。
// 原因：① 替换 bundle 内二进制会破坏代码签名与 Info.plist 等资源；
//      ② App Translocation / DMG 只读卷等场景下对可执行文件 rename 必然失败且被静默吞掉。

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func IsRunningAsAdmin() bool {
	return true
}

func (a *App) RestartAsAdmin() error {
	return nil
}

// currentAppBundle 返回当前运行的 .app bundle 根目录与可执行文件路径。
// 非法运行环境（开发模式等）返回错误。
func currentAppBundle() (bundleRoot, exePath string, err error) {
	exePath, err = os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("无法确定当前程序路径: %w", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	// App Translocation：带 quarantine 属性且未移入 /Applications 的 app，
	// macOS 会在只读的随机路径 /private/var/folders/.../AppTranslocation/... 下运行
	if strings.Contains(exePath, "AppTranslocation") {
		return "", "", errors.New("检测到应用正从临时隔离位置运行。请先将 go-stock 拖入「应用程序」文件夹，重新打开后再检查更新")
	}
	// 安装镜像（DMG）为只读卷
	if strings.HasPrefix(exePath, "/Volumes/") {
		return "", "", errors.New("检测到应用正从安装镜像(DMG)中运行。请先将 go-stock 拖入「应用程序」文件夹，重新打开后再检查更新")
	}

	// Contents/MacOS/exe → 上两级即 bundle 根
	bundleRoot = filepath.Dir(filepath.Dir(exePath))
	if !strings.HasSuffix(bundleRoot, ".app") {
		return "", "", errors.New("当前运行环境不是 .app 安装包（可能是开发模式），请手动下载更新")
	}
	return bundleRoot, exePath, nil
}

// ApplyMacUpdate 用下载的 .app bundle zip 整体替换当前应用。
// 替换采用同卷 rename 原子交换：旧 bundle → .old，新 bundle → 原位；失败自动回滚。
// 替换成功后不强制重启（不打断用户），用户下次打开应用即加载新版本。
func ApplyMacUpdate(zipPath string) error {
	bundleRoot, _, err := currentAppBundle()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "go-stock-update-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	newBundle, err := unzipAppBundle(zipPath, tmpDir)
	if err != nil {
		return err
	}

	oldBundle := bundleRoot + ".old"
	_ = os.RemoveAll(oldBundle) // 清理上次更新残留
	if err = os.Rename(bundleRoot, oldBundle); err != nil {
		return fmt.Errorf("移动旧版本失败（可能没有应用程序目录的写入权限）: %w", err)
	}
	if err = os.Rename(newBundle, bundleRoot); err != nil {
		_ = os.Rename(oldBundle, bundleRoot) // 回滚
		return fmt.Errorf("放置新版本失败: %w", err)
	}

	// 当前进程仍从旧 inode 运行，延迟删除旧 bundle 不影响使用
	go func() {
		time.Sleep(10 * time.Second)
		_ = os.RemoveAll(oldBundle)
	}()
	return nil
}

// unzipAppBundle 解压 zip 到 destDir，返回其中 .app bundle 的路径。
// 保留文件权限（Mach-O 可执行位）与符号链接。
func unzipAppBundle(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开更新包失败: %w", err)
	}
	defer r.Close()

	var appDirName string
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		// 拒绝路径穿越
		if strings.Contains(name, "..") {
			continue
		}
		if appDirName == "" {
			if first := strings.SplitN(strings.TrimPrefix(name, "./"), "/", 2)[0]; strings.HasSuffix(first, ".app") {
				appDirName = first
			}
		}

		target := filepath.Join(destDir, name)
		mode := f.FileInfo().Mode()

		switch {
		case mode&os.ModeSymlink != 0:
			linkTarget, err := readZipEntry(f)
			if err != nil {
				return "", err
			}
			_ = os.Remove(target)
			if err = os.Symlink(linkTarget, target); err != nil {
				return "", fmt.Errorf("创建符号链接失败 %s: %w", name, err)
			}
		case f.FileInfo().IsDir():
			if err = os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		default:
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			if err = extractZipEntry(f, target, mode); err != nil {
				return "", err
			}
		}
	}
	if appDirName == "" {
		return "", errors.New("更新包中未找到 .app 应用程序")
	}
	return filepath.Join(destDir, appDirName), nil
}

func readZipEntry(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	buf, err := io.ReadAll(io.LimitReader(rc, 4096))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func extractZipEntry(f *zip.File, target string, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("写入 %s 失败: %w", target, err)
	}
	rc, err := f.Open()
	if err != nil {
		out.Close()
		return err
	}
	_, err = io.Copy(out, rc)
	rc.Close()
	out.Close()
	return err
}

// CleanOldMacBundle 清理更新残留的 .old bundle（startup 兜底调用，
// 覆盖替换成功后 10 秒延迟删除未来得及执行、进程被提前退出的场景）
func CleanOldMacBundle() {
	bundleRoot, _, err := currentAppBundle()
	if err != nil {
		return
	}
	_ = os.RemoveAll(bundleRoot + ".old")
}
