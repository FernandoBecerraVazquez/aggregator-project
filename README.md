# Aggregator

*Postgres and Go are required to be installed to run this program.*

**Gator** is an RSS feed agregator built using Go!

## Functions ##
- Add RSS feeds from across the internet to be collected.
- Store the collected posts in a PostgreSQL database.
- Follow and unfollow RSS feeds that other users have added.
- View summaries of the aggregated posts in the terminal, with a link to the full post.

To install the gator CLI run `go install gator` in the terminal.

## Create config file ##
1. Create a config file in your home directory, `~/.gatorconfig.json`, with the following content:
```
# content
{
  "db_url": "postgres://example"
}
```

2. In a new project directory, initialize a new Go module using go mod init. Create a new file, main.go. Add a main function. It doesn't need to do anything yet.

3. Create the internal and internal/config directories to contain the config internal package. This package will be responsible for reading and writing the JSON file.

This package should have the following functionality exported so the main package can use it:

    * Export a Config struct that represents the JSON file structure, including struct tags.
    ```
    type Config struct {
        DbUrl           string `json:"db_url"`
        CurrentUserName string `json:"current_user_name"`
    }
    ```
    * Export a Read function that reads the JSON file found at ~/.gatorconfig.json and returns a Config struct. It should read the file from the HOME directory, then decode the JSON string into a new Config struct.
    * This is what I did to get the the location of HOME:*
    ```
    const configFileName = ".gatorconfig.json"

    func getConfigFilePath() (string, error) {
        path, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("failed to get user home directory: %w", err)
        }
        return filepath.Join(path, configFileName), nil
    }
    ```
    * Export a SetUser method on the Config struct that writes the config struct to the JSON file after setting the current_user_name field.

    * I also used a non-exported helper function called write*
    ```
    func write(cfg Config) error {
	path, err := getConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
    }
    ```

4. Update the main function to:
    * Read the config file.
    * Set the current user to your name and update the config file on disk.
    * Read the config file again and print the contents of the config struct to the terminal.

## SQLC ##
1. Install SQLC. `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`, Then run sqlc version to make sure it's installed correctly.
2. Configure SQLC. You'll always run the sqlc command from the root of your project. Create a file called sqlc.yaml in the root of your project.
    ```
    version: "2"
    sql:
    - schema: "sql/schema"
        queries: "sql/queries"
        engine: "postgresql"
        gen:
        go:
            out: "internal/database"
    ```
3. Write a query to create a user. Inside the sql/queries directory, create a file called users.sql. Here's the format:
    ```
    -- name: CreateUser :one
    INSERT INTO users (id, created_at, updated_at, name)
    VALUES (
        $1,
        $2,
        $3,
        $4
    )
    RETURNING *;
    ```
4. Generate the Go code. Run sqlc generate from the root of your project. It should create a new package of go code in internal/database. *You need to install Google's uuid package module `go get github.com/google/uuid`*
5. Write another query to get a user by name. I used the comment -- name: GetUser :one to tell SQLC that I expect to get back a single row. Again, generate the Go code to ensure that it works.
6. Import a PostgreSQL driver. *Install in your module `go get github.com/lib/pq`*
    Add this import to the top of your main.go file `import _ "github.com/lib/pq"`
7. Open a connection to the database, and store it in the state struct
    In main(), load in your database URL to the config struct and sql.Open() a connection to your database:
    `db, err := sql.Open("postgres", dbURL)`
    Use your generated database package to create a new *database.Queries, and store it in your state struct:
    `dbQueries := database.New(db)`
    ```
    type state struct {
    db  *database.Queries
    cfg *config.Config
    }
    ```
8. Create a register handler and register it with the commands. Usage:
    `go run . register *name*`
    Use psql to verify that the user was created:
    * Mac: psql gator
    * Linux: sudo -u postgres psql gator

    `SELECT * FROM users;`

9. Update the login command handler to error (and exit with code 1) if the given username doesn't exist in the database. You can't login to an account that doesn't exist!

## Commands ##
* gator login - sets the current user in the config
* gator register - adds a new user to the database
* gator users - lists all the users in the database
* add feed - adds a new feed to the database
* gator follow - follow a feed created by other users
* gator following - check what feeds is the current user is following



