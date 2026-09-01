// Package sqlite3 是 github.com/mattn/go-sqlite3 的空壳替换（见主仓库 go.mod 的 replace 指令）。
//
// go-stock 的 sqlite 驱动统一使用 modernc.org/sqlite（与依赖 gotdx 共用同一实现），
// 该驱动注册 database/sql 驱动名 "sqlite"，由 backend/db 在打开连接时显式指定。
// gorm 官方驱动 gorm.io/driver/sqlite 会 blank-import mattn/go-sqlite3（CGO 实现，
// 驱动名 "sqlite3"），本空壳包将其顶替：不注册驱动、不引入 CGO，保持纯 Go 构建。
package sqlite3
