// source file path: ./src/common/jwt/v-jwt.go
package vjwt

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"os"
	"time"
)

var pk = os.Getenv("vibe_pk")

//json:easyjson
type CustomClaims struct {
	*jwt.StandardClaims
	UserID   string `json:"UserId"`
	DeviceID string `json:"DeviceId"`
}

func CreateToken(userId string, deviceId string) (string, error) {

	// Create the Claims
	claims := &CustomClaims{
		StandardClaims: &jwt.StandardClaims{
			ExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "vibeirl.com",
			Subject:   "vibe-jwt-claims",
		},
		UserID:   userId,
		DeviceID: deviceId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign and get the complete encoded token as a string
	tokenString, err := token.SignedString(pk)

	if err != nil {
		vbl.Stdout.Error("vid/b2ba1c7b106f", "Error signing token:", err)
		return "", err
	}

	return tokenString, nil
}

func ReadJWT(tokenString string) (*CustomClaims, error) {
	// Parse the JWT, using the same signing method and key as when it was created
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("vid/a473f9d493de: unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(pk), nil
	})

	if err != nil {
		vbl.Stdout.Error("vid/49b00bd7522f", "Error verifying JWT: %v", err)
		return nil, err
	}

	// Assert the type of the claims and check if the token is valid
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		// Additional validation of claims can be done here
		if err := claims.Valid(); err != nil {
			vbl.Stdout.Error("vid/b6a94a47a9da", "invalid token")
			return nil, err
		}
		return claims, nil
	} else {
		vbl.Stdout.Error("vid/a761198d0c7c", "invalid token")
		return nil, fmt.Errorf("could not validate token or wrong claim types")
	}
}
