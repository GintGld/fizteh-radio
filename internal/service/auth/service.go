package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"

	"golang.org/x/crypto/bcrypt"
)

// TODO: enable refesh token

type Auth struct {
	log           *slog.Logger
	editorStorage EditorStorage
	jwtMaker      jwtMaker
	rootPassHash  []byte
	tokenTTL      time.Duration
}

type jwtMaker interface {
	NewToken(editor models.Editor, duration time.Duration) (string, error)
}

type EditorStorage interface {
	EditorByLogin(ctx context.Context, login string) (models.Editor, error)
}

// New returns new instance of authentication service
func New(
	log *slog.Logger,
	editorStorage EditorStorage,
	jwtMaker jwtMaker,
	rootPassHash []byte,
	tokenTTL time.Duration,
) *Auth {
	return &Auth{
		log:           log,
		editorStorage: editorStorage,
		jwtMaker:      jwtMaker,
		rootPassHash:  rootPassHash,
		tokenTTL:      tokenTTL,
	}
}

// Login checks if editor with given credentials exists in the system and returns access token.
//
// If editor exists, but password is incorrect, returns error.
// If editor doesn't exist, returns error.
func (a *Auth) Login(ctx context.Context, login string, password string) (string, error) {
	const op = "Auth.Login"

	var token string
	var err error

	if login == models.RootLogin {
		token, err = a.loginRoot(ctx, password)
		if err != nil {
			return "", fmt.Errorf("%s: %w", op, err)
		}
	} else {
		token, err = a.loginEditor(ctx, login, password)
		if err != nil {
			return "", fmt.Errorf("%s: %w", op, err)
		}
	}

	return token, nil
}

func (a *Auth) loginRoot(_ context.Context, password string) (string, error) {
	const op = "Auth.login.Root"

	log := a.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("attempting to login root")

	if err := bcrypt.CompareHashAndPassword(a.rootPassHash, []byte(password)); err != nil {
		log.Info("invalid credentials", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, service.ErrInvalidCredentials)
	}

	log.Info("root logged successfully")

	token, err := a.jwtMaker.NewToken(models.Editor{ID: models.RootID, Login: models.RootLogin}, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (a *Auth) loginEditor(ctx context.Context, login string, password string) (string, error) {
	const op = "Auth.loginEditor"

	log := a.log.With(
		slog.String("op", op),
		slog.String("editorname", login),
	)

	log.Info("attempting to login editor")

	editor, err := a.editorStorage.EditorByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, storage.ErrEditorNotFound) {
			log.Warn("editor not found", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, service.ErrInvalidCredentials)
		}

		log.Error("failed to get editor", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(editor.PassHash, []byte(password)); err != nil {
		log.Info("invalid credentials", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, service.ErrInvalidCredentials)
	}

	log.Info("editor logged in successfully")

	token, err := a.jwtMaker.NewToken(editor, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}
