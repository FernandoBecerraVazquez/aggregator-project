package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/FernandoBecerraVazquez/Aggregator/internal/config"
	"github.com/FernandoBecerraVazquez/Aggregator/internal/database"
)

func main() {
	data, err := config.Read()
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	sqlDB, err := sql.Open("postgres", data.DbUrl)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	dbQueries := database.New(sqlDB)

	s := &state{c: &data, db: dbQueries}

	configCommands := make(map[string]func(*state, command) error)

	cmds := commands{
		Commands: configCommands,
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

	if len(os.Args) < 2 {
		fmt.Println("error: command is required")
		os.Exit(1)
	}

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	err = cmds.run(s, cmd)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

}
