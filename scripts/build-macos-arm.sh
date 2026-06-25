#! /bin/bash

echo -e "Start running the script..."
cd ../

# 通过环境变量注入 BuildKey / Version，避免使用默认占位 BuildKey
# （默认 key 会导致导入用官方 key 加密的赞助码时解密失败）。
# 用法示例：BUILD_KEY=你的key ./scripts/build-macos-arm.sh
LDFLAGS=""
if [ -n "$BUILD_KEY" ]; then
  LDFLAGS="$LDFLAGS -X main.BuildKey=$BUILD_KEY"
fi
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null)
if [ -n "$GIT_COMMIT" ]; then
  LDFLAGS="$LDFLAGS -X main.VersionCommit=$GIT_COMMIT"
fi
if [ -n "$APP_VERSION" ]; then
  LDFLAGS="$LDFLAGS -X main.Version=$APP_VERSION"
fi

echo -e "Start building the app for macos platform..."
if [ -n "$LDFLAGS" ]; then
  echo -e "Injecting ldflags (BuildKey/Version present: $([ -n "$BUILD_KEY" ] && echo yes || echo no))"
  wails build --clean --platform darwin/arm64 -ldflags "$LDFLAGS"
else
  wails build --clean --platform darwin/arm64
fi

echo -e "End running the script!"

