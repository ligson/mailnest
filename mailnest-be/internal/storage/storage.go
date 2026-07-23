package storage

type Store struct {
	db *database
}

// Open 使用默认 SQLite 配置打开存储层，主要服务本地开发和测试。
func Open(path string) (*Store, error) {
	return OpenWithOptions(DatabaseOptions{
		Driver: "sqlite",
		Path:   path,
	})
}

// OpenWithOptions 根据配置打开 SQLite/MySQL/PostgreSQL，并执行启动迁移。
func OpenWithOptions(options DatabaseOptions) (*Store, error) {
	db, err := openDatabase(options)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// migrate 对已有 SQLite 生产库走兼容迁移，新库或其他数据库走 GORM 迁移。
func (s *Store) migrate() error {
	if s.db.dialect == dialectSQLite {
		exists, err := s.sqliteTableExists("users")
		if err != nil {
			return err
		}
		if exists {
			return s.migrateExistingSQLite()
		}
	}
	return s.migrateGORM()
}
