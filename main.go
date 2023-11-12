package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"fmt"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var SECRET_KEY = []byte("gosecretkey")

type User struct {
	Name     string `json:"name" bson:"name"`
	Mobile   string `json:"mobile" bson:"mobile"`
	Password string `json:"password" bson:"password"`
	Address  string `json:"address" bson:"address"`
}

var client *mongo.Client

func getHash(pwd []byte) string {
	hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.MinCost)
	if err != nil {
		log.Println(err)
	}
	return string(hash)
}

func GenerateJWT() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, err := token.SignedString(SECRET_KEY)
	if err != nil {
		log.Println("Error in JWT token generation")
		return "", err
	}
	return tokenString, nil
}

func userSignup(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	var user User
	json.NewDecoder(request.Body).Decode(&user)
	user.Password = getHash([]byte(user.Password))
	collection := client.Database("GODB").Collection("user")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	result, _ := collection.InsertOne(ctx, user)
	json.NewEncoder(response).Encode(result)
}

func userLogin(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	var user User
	var dbUser User
	json.NewDecoder(request.Body).Decode(&user)
	collection := client.Database("GODB").Collection("user")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err := collection.FindOne(ctx, bson.M{"mobile": user.Mobile}).Decode(&dbUser)

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message":"` + err.Error() + `"}`))
		return
	}
	userPass := []byte(user.Password)
	dbPass := []byte(dbUser.Password)

	passErr := bcrypt.CompareHashAndPassword(dbPass, userPass)

	if passErr != nil {
		log.Println(passErr)
		response.Write([]byte(`{"response":"Wrong Password!"}`))
		return
	}
	jwtToken, err := GenerateJWT()
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"message":"` + err.Error() + `"}`))
		return
	}
	response.Write([]byte(`{"token":"` + jwtToken + `"}`))

}

type Election struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	Name       string             `bson:"name"`
	Date       string             `bson:"date"`
	Candidates []Candidate        `bson:"candidates"`
}

type Candidate struct {
	ID    string `bson:"id"`
	Name  string `bson:"name"`
	Votes int    `bson:"votes"`
}

func GetElections(w http.ResponseWriter, r *http.Request) {
	var elections []Election
	cursor, err := client.Database("GODB").Collection("election").Find(context.TODO(), bson.M{})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = cursor.All(context.TODO(), &elections)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(elections)
}

// GetElectionDetails returns details of a specific election
func GetElectionDetails(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	electionID := params["id"]

	var election Election
	id, err := ObjectIDFromHex(electionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	cur := client.Database("GODB").Collection("election").FindOne(context.TODO(), bson.M{
		"_id": id,
	}).Decode(&election)
	if cur != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(election)
}
func ObjectIDFromHex(s string) (primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return objID, nil
}

// VoteForCandidate increments the vote count for a specific candidate in a specific election
func VoteForCandidate(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	electionID := params["electionID"]
	candidateID := params["candidateID"]

	objID, err := primitive.ObjectIDFromHex(electionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updateResult, err := client.Database("GODB").Collection("election").UpdateOne(
		context.TODO(),
		bson.M{"_id": objID, "candidates.id": candidateID},
		bson.M{"$inc": bson.M{"candidates.$.votes": 1}},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if updateResult.ModifiedCount == 0 {

		http.Error(w, "Candidate or Election not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Vote counted successfully!")
}
func electioninsert(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	var electionin Election
	json.NewDecoder(request.Body).Decode(&electionin)
	collection := client.Database("GODB").Collection("election")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	result, _ := collection.InsertOne(ctx, electionin)
	json.NewEncoder(response).Encode(result)
}

func main() {
	log.Println("Starting the application")

	r := mux.NewRouter()
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, _ = mongo.Connect(ctx, options.Client().ApplyURI("mongodb+srv://Malavika:malavika@cluster0.lv45wuq.mongodb.net/GODB?retryWrites=true&w=majority"))

	r.HandleFunc("/index", userLogin).Methods("POST")
	r.HandleFunc("/register", userSignup).Methods("POST")
	r.HandleFunc("/electionin", userSignup).Methods("POST")

	r.HandleFunc("/elections", GetElections).Methods("GET")
	r.HandleFunc("/elections/{id}", GetElectionDetails).Methods("GET")
	r.HandleFunc("/vote/{electionID}/{candidateID}", VoteForCandidate).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", r))

}
