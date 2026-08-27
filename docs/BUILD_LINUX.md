# go-stock Linux 构建指南

本文档说明如何在 Linux 平台上编译和运行 go-stock 应用程序。

## 系统要求

- **操作系统**: Linux (推荐 Ubuntu 20.04+ 或 Debian 11+)
- **Go**: 1.21 或更高版本
- **Node.js**: 18.0 或更高版本
- **npm**: 9.0 或更高版本
- **Wails**: v2.11.0 或更高版本
- **系统依赖**: libgtk-3-dev, libwebkit2gtk-4.0-dev

## 方法一：直接在 Linux 上构建

### 1. 安装系统依赖

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y \
    libgtk-3-dev \
    libwebkit2gtk-4.0-dev \
    gcc \
    libglib2.0-dev
```

**Fedora/RHEL:**
```bash
sudo dnf install -y \
    gtk3-devel \
    webkit2gtk3-devel \
    gcc \
    glib2-devel
```

**Arch Linux:**
```bash
sudo pacman -S \
    gtk3 \
    webkit2gtk \
    gcc \
    glib2
```

### 2. 安装 Go

从 [Go 官网](https://go.dev/dl/) 下载并安装最新版本，或使用版本管理工具：

```bash
# 使用 gvm
curl -sSL https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer.sh | bash
gvm install go1.21
gvm use go1.21
```

### 3. 安装 Node.js 和 npm

```bash
# 使用 nvm (推荐)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 18
nvm use 18
```

### 4. 安装 Wails

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 5. 构建应用

```bash
# 克隆项目
git clone https://github.com/ArvinLovegood/go-stock.git
cd go-stock

# 运行构建脚本
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh
```

构建完成后，可执行文件位于 `build/bin/linux/go-stock`

### 6. 运行应用

```bash
./build/bin/linux/go-stock
```

## 方法二：使用 Docker 构建

如果您希望在任何平台（Windows、macOS）上构建 Linux 版本，可以使用 Docker。

### 1. 安装 Docker

按照 [Docker 官方文档](https://docs.docker.com/get-docker/) 安装 Docker。

### 2. 运行 Docker 构建脚本

```bash
# Windows (PowerShell 或 Git Bash)
.\scripts\docker-build-linux.sh

# macOS / Linux
chmod +x scripts/docker-build-linux.sh
./scripts/docker-build-linux.sh
```

### 3. 运行 Docker 容器

```bash
docker run --rm -it \
    -e DISPLAY=$DISPLAY \
    -v /tmp/.X11-unix:/tmp/.X11-unix \
    go-stock-linux-builder
```

## 方法三：使用 Wails 开发模式

在 Linux 上进行开发时，可以使用开发模式：

```bash
# 安装依赖
wails doctor

# 运行开发模式
wails dev
```

## 常见问题

### 1. 缺少 libwebkit2gtk-4.0-dev

错误信息：
```
error: failed to import C source code:
```

解决方案：
```bash
sudo apt-get install libwebkit2gtk-4.0-dev
```

### 2. 缺少 libgtk-3-dev

错误信息：
```
gtk/gtk.h: No such file or directory
```

解决方案：
```bash
sudo apt-get install libgtk-3-dev
```

### 3. 运行时提示找不到 WebView2

这是正常的，因为 Linux 使用 WebKitGTK 而不是 WebView2。确保已安装：
```bash
sudo apt-get install libwebkit2gtk-4.0-37
```

### 4. 显示或字体问题

如果应用程序界面显示异常，尝试安装中文字体：
```bash
sudo apt-get install fonts-wqy-zenhei fonts-wqy-microhei
```

## 打包分发

### 创建 AppImage (推荐)

```bash
# 安装 appimagetool
wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x appimagetool-x86_64.AppImage

# 创建 AppDir 结构
mkdir -p AppDir/usr/bin
mkdir -p AppDir/usr/share/applications
mkdir -p AppDir/usr/share/icons/hicolor/256x256/apps

# 复制文件
cp build/bin/linux/go-stock AppDir/usr/bin/
cp build/appicon.png AppDir/usr/share/icons/hicolor/256x256/apps/go-stock.png

# 创建 AppRun
echo '#!/bin/bash
DIR="$(dirname "$(readlink -f "${0}")")"
exec "${DIR}/usr/bin/go-stock" "$@"' > AppDir/AppRun
chmod +x AppDir/AppRun

# 创建 desktop 文件
echo '[Desktop Entry]
Type=Application
Name=go-stock
Exec=go-stock
Icon=go-stock
Categories=Finance;' > AppDir/usr/share/applications/go-stock.desktop

# 生成 AppImage
./appimagetool-x86_64.AppImage AppDir go-stock.AppImage
```

### 创建 deb 包

```bash
# 安装 dpkg-deb
sudo apt-get install dpkg-dev

# 创建 deb 包结构
mkdir -p go-stock_DEBIAN
mkdir -p go-stock/usr/bin
mkdir -p go-stock/usr/share/applications
mkdir -p go-stock/usr/share/icons/hicolor/256x256/apps

# 创建 control 文件
echo 'Package: go-stock
Version: 1.0.0
Architecture: amd64
Maintainer: sparkmemory
Depends: libgtk-3-0, libwebkit2gtk-4.0-37
Description: AI 赋能股票分析软件' > go-stock_DEBIAN/control

# 复制文件
cp build/bin/linux/go-stock go-stock/usr/bin/
cp build/appicon.png go-stock/usr/share/icons/hicolor/256x256/apps/go-stock.png

# 创建 desktop 文件
echo '[Desktop Entry]
Type=Application
Name=go-stock
Exec=/usr/bin/go-stock
Icon=go-stock
Categories=Finance;' > go-stock/usr/share/applications/go-stock.desktop

# 构建 deb 包
dpkg-deb --build go-stock go-stock.deb
```

## 技术支持

如有问题，请提交 Issue 至：
https://github.com/ArvinLovegood/go-stock/issues

## 许可证

详见项目根目录的 LICENSE 文件。
