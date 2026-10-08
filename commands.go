package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/FernandoBecerraVazquez/Aggregator/internal/config"
	"github.com/FernandoBecerraVazquez/Aggregator/internal/database"
)

type state struct {
	db *database.Queries
	c  *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	Commands map[string]func(*state, command) error
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	name := cmd.args[0]
	_, err := s.db.GetUser(context.Background(), name)
	if err != nil {
		return err
	}

	err = s.c.SetUser(name)
	if err != nil {
		return err
	}
	fmt.Printf("Logged in as: %s\n", s.c.CurrentUserName)
	return nil
}

func (c *commands) run(s *state, cmd command) error {
	if handler, exists := c.Commands[cmd.name]; exists {
		return handler(s, cmd)
	}
	return fmt.Errorf("unknown command: %s", cmd.name)
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.Commands[name] = f
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	name := cmd.args[0]
	now := time.Now()

	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
	}

	user, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}
	err = s.c.SetUser(name)
	if err != nil {
		return err
	}
	fmt.Printf("User created: %s (ID: %s)\n", user.Name, user.ID)
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.DeleteUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error resetting users table: %w", err)
	}

	fmt.Println("Reset Successful")
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		currentUser, err := s.db.GetUser(context.Background(), s.c.CurrentUserName)
		if err != nil {
			return fmt.Errorf("error getting current user: %w", err)
		}
		return handler(s, cmd, currentUser)
	}
}

func handlerUsers(s *state, cmd command) error {
	userList, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error retrieving users from table: %w", err)
	}

	for _, name := range userList {
		if name == s.c.CurrentUserName {
			fmt.Printf("* %s (current)\n", name)
		} else {
			fmt.Printf("* %s\n", name)
		}
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("time interval is required")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("invalid time interval")
	}

	fmt.Printf("Collecting feeds every %v\n", timeBetweenRequests)

	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("feed URL is required")
	}

	feedName := cmd.args[0]
	url := cmd.args[1]

	feedParams := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      feedName,
		Url:       url,
		UserID:    user.ID,
	}

	feed, err := s.db.CreateFeed(context.Background(), feedParams)
	if err != nil {
		return fmt.Errorf("error creating feed: %w", err)
	}

	feedFollowParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		FeedID:    feed.ID,
		UserID:    user.ID,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return fmt.Errorf("error creating feed follow: %w", err)
	}

	fmt.Printf("Feed created: %s (ID: %s)\n Url: %s\n User ID: %s\n", feed.Name, feed.ID, feed.Url, feed.UserID)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feedsList, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error retrieving feeds from table: %w", err)
	}

	for _, feed := range feedsList {
		fmt.Printf("%s\n %s\n %v\n", feed.Name, feed.Url, feed.UserName)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("feed URL is required")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return fmt.Errorf("error getting feed by URL: %w", err)
	}

	feedFollowParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		FeedID:    feed.ID,
		UserID:    user.ID,
	}

	feedFollow, err := s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return fmt.Errorf("error creating feed follow: %w", err)
	}

	fmt.Printf("Feed Follow: %s\n %s\n", feedFollow.FeedName, feedFollow.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {

	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error retrieving feed follows: %w", err)
	}

	for _, follow := range feedFollows {
		fmt.Printf("%s\n", follow.FeedName)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("feed URL is required")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return fmt.Errorf("error getting feed by URL: %w", err)
	}

	feedParams := database.DeleteFeedFollowParams{
		FeedID: feed.ID,
		UserID: user.ID,
	}

	err = s.db.DeleteFeedFollow(context.Background(), feedParams)
	if err != nil {
		return fmt.Errorf("error deleting feed follow: %w", err)
	}

	fmt.Printf("Unfollowed feed: %s\n", feed.Name)
	return nil
}

func scrapeFeeds(s *state) {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		fmt.Printf("Error getting next feed to fetch: %v\n", err)
		return
	}

	feedFetchedParams := database.MarkFeedFetchedParams{
		LastFetchedAt: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
		UpdatedAt: time.Now(),
		ID:        nextFeed.ID,
	}

	err = s.db.MarkFeedFetched(context.Background(), feedFetchedParams)
	if err != nil {
		fmt.Printf("Error marking feed as fetched: %v\n", err)
	}

	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		fmt.Printf("Error fetching feed: %v\n", err)
		return
	}

	for _, item := range feed.Channel.Item {
		var publishedAt sql.NullTime
		parsedTime, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			publishedAt = sql.NullTime{Valid: false}
		} else {
			publishedAt = sql.NullTime{Time: parsedTime, Valid: true}
		}

		postParams := database.CreatePostParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Title:     item.Title,
			Url:       item.Link,
			Description: sql.NullString{
				String: item.Description,
				Valid:  item.Description != "",
			},
			PublishedAt: publishedAt,
			FeedID:      nextFeed.ID,
		}

		_, err = s.db.CreatePost(context.Background(), postParams)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				continue
			} else {
				fmt.Printf("Error creating post: %v\n", err)
				continue
			}
		}
	}
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	var err error
	if len(cmd.args) == 1 {
		limit, err = strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("invalid limit: %w", err)
		}
	}

	userPostParams := database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	}

	posts, err := s.db.GetPostsForUser(context.Background(), userPostParams)
	if err != nil {
		return fmt.Errorf("error retrieving posts for user: %w", err)
	}

	for _, post := range posts {
		fmt.Printf("Title: %s\nURL: %s\nDescription: %s\nPublished At: %v\nFeed Name: %s\n\n",
			post.Title, post.Url, post.Description.String, post.PublishedAt.Time, post.FeedName)
	}
	return nil
}
