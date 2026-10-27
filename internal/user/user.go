package user

import (
	"context"
	"database/sql"
	"errors"
	"io"
)

type User struct {
	ID    int64
	Name  string
	Email string
}

func ListUsers(
	ctx context.Context,
	db *sql.DB,
	limit int,
) ([]User, error) {

	usersList := make([]User, 0, limit)
	query := `SELECT id, name, email FROM users LIMIT $1`
	rows, err := db.QueryContext(ctx, query, limit)

	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		err := rows.Scan(&user)
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		usersList = append(usersList, user)
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}

	return usersList, nil
}
