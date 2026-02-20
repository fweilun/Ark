// README: Firebase Admin SDK initialisation and token verifier.
package infra

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// FirebaseToken holds the verified token data used by downstream middleware.
type FirebaseToken struct {
	UID    string
	Claims map[string]interface{}
}

// TokenVerifier verifies a raw Firebase ID token string and returns token data.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*FirebaseToken, error)
}

// firebaseVerifier is the production implementation backed by the Firebase Admin SDK.
type firebaseVerifier struct {
	client *auth.Client
}

// NewFirebaseVerifier creates a TokenVerifier using the Firebase Admin SDK.
// If credentialsFile is non-empty it is used as the service-account JSON path;
// otherwise application-default credentials / GOOGLE_APPLICATION_CREDENTIALS are used.
// projectID is required so the SDK can construct the correct token-verification URL.
func NewFirebaseVerifier(ctx context.Context, projectID, credentialsFile string) (TokenVerifier, error) {
	opts := []option.ClientOption{}
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, opts...)
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase app.Auth: %w", err)
	}
	return &firebaseVerifier{client: client}, nil
}

func (v *firebaseVerifier) VerifyIDToken(ctx context.Context, idToken string) (*FirebaseToken, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	return &FirebaseToken{UID: token.UID, Claims: token.Claims}, nil
}
