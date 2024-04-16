package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/joho/godotenv"
)

// User represents the user data to be stored in the database
type User struct {
	gorm.Model
	TelegramID                 int64  `gorm:"unique;not null"`
	Username                   string `gorm:"unique"`
	Name                       string
	MobileNumber               string
	EnglishLevel               string
	Gender                     string
	MediaID                    uint
	Latitude                   float64
	Longitude                  float64
	CurrentQuestion            int       // Added field to track the current question
	CurrentFindPartnerQuestion int       // Added field to track the current find partner feature question
	LastSelectedEnglishLevel   string    // Added filed for store last selected english level filter
	LastSelectedGender         string    // Added filed for store last selected gender filter
	LastFindPartnerTime        time.Time // Added field to store last time find partner
	CountWatchPartnerLimit     int       // Added for store count of watch user partner from limited partners list in each day (24 hours)
	CurrentNumberInPartnerList int       // Added for store current number of partners search in json data
	CurrentEditProfileQuestion string    // Added for store current user edit profile question as string
	// Add the following relationship for follow requests
	FollowRequestsSent     []FollowRequest `gorm:"foreignkey:RequesterID"`
	FollowRequestsReceived []FollowRequest `gorm:"foreignkey:TargetID"`
}

type Media struct {
	ID        uint `gorm:"primary_key"`
	Filename  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FollowRequest struct {
	gorm.Model
	RequesterID int64 // ID of the user sending the follow request
	TargetID    int64 // ID of the user being followed
	Accepted    bool  // Indicates whether the follow request is accepted
}

type WatchList struct {
	ID      uint
	UserID  int64 // ID of the user watched
	WatchID int64 // ID of the user seen by above user_id
}

var db *gorm.DB
var err error
var redisClient *redis.Client

const (
	QuestionName = iota
	QuestionMobileNumber
	QuestionEnglishLevel
	QuestionProfilePhoto
	QuestionGender
	// QuestionLocation
	QuestionDone
)

// Add the following constants to define the number of users to show and the time limit
const (
	UsersToShowLimit = 10
	WaitTimeLimit    = 24 * time.Hour
)

var mainKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🤜🤛👥 Find Partner"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🧑‍💼 Show Profile"),
		tgbotapi.NewKeyboardButton("🧑‍💼🛠️ Edit Profile"),
	),
)

var englishLevelKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Beginner"),
		tgbotapi.NewKeyboardButton("Intermediate"),
		tgbotapi.NewKeyboardButton("Advanced"),
	),
)

var selectGenderKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👨 Male"),
		tgbotapi.NewKeyboardButton("👩 Female"),
	),
)

var selectGenderFilterKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👨 Male"),
		tgbotapi.NewKeyboardButton("👩 Female"),
		tgbotapi.NewKeyboardButton("🤷‍♂️ Does Not Matter"),
	),
)

var selectNextOrAcceptPartnerKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("✅ Follow Partner"),
		tgbotapi.NewKeyboardButton("➡️ Next Partner"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🏠 Back To Home Menu"),
	),
)

var backToHomeMenuKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🏠 Back To Home Menu"),
	),
)

var iDoNotWantToEnterMobileMenuKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("⏭️ I do not want to enter mobile number"),
	),
)

var editProfileMenuKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👤 Edit Name"),
		tgbotapi.NewKeyboardButton("🗣️🌍 Edit English Level"),
		tgbotapi.NewKeyboardButton("👫 Edit Gender"),
		tgbotapi.NewKeyboardButton("🖼️ Edit Profile Photo"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🏠 Back To Home Menu"),
	),
)

var editEnglishLevelKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Beginner"),
		tgbotapi.NewKeyboardButton("Intermediate"),
		tgbotapi.NewKeyboardButton("Advanced"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👤 Edit Name"),
		tgbotapi.NewKeyboardButton("🗣️🌍 Edit English Level"),
		tgbotapi.NewKeyboardButton("👫 Edit Gender"),
		tgbotapi.NewKeyboardButton("🖼️ Edit Profile Photo"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🏠 Back To Home Menu"),
	),
)

var editGenderKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👨 Male"),
		tgbotapi.NewKeyboardButton("👩 Female"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("👤 Edit Name"),
		tgbotapi.NewKeyboardButton("🗣️🌍 Edit English Level"),
		tgbotapi.NewKeyboardButton("👫 Edit Gender"),
		tgbotapi.NewKeyboardButton("🖼️ Edit Profile Photo"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🏠 Back To Home Menu"),
	),
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Panicf("could not load env: %v\n", err)
	}
	apikey := os.Getenv("TELEGRAM_APITOKEN")

	db, err = gorm.Open("postgres", "host=localhost user=postgres dbname=partner_go sslmode=disable password=987654321")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// redis cache server
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Replace with your Redis server address
		Password: "",               // No password by default
		DB:       0,                // Default DB
	})
	// Test the Redis connection
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}

	// AutoMigrate creates tables based on the User struct
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Media{})
	db.AutoMigrate(&FollowRequest{})
	db.AutoMigrate(&WatchList{})

	// Replace "YOUR_BOT_TOKEN" with your actual bot token
	bot, err := tgbotapi.NewBotAPI(apikey)
	if err != nil {
		log.Panic(err)
	}

	// Use webhook or long polling based on your deployment environment
	// For simplicity, we are using long polling here
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			// Check if the callback data starts with "accept_follow:"
			if strings.HasPrefix(update.CallbackQuery.Data, "accept_follow:") {
				partnerID, err := strconv.ParseInt(strings.TrimPrefix(update.CallbackQuery.Data, "accept_follow:"), 10, 64)
				if err != nil {
					log.Println("Error parsing partner ID on accept_follow:", err)
				}

				// Call the handleAcceptFollow function
				handleAcceptFollow(bot, update, partnerID)
			} else if strings.HasPrefix(update.CallbackQuery.Data, "decline_follow:") {
				partnerID, err := strconv.ParseInt(strings.TrimPrefix(update.CallbackQuery.Data, "decline_follow:"), 10, 64)
				if err != nil {
					log.Println("Error parsing partner ID on decline follow:", err)
				}

				// Call the handleAcceptFollow function
				handleDeclineFollow(bot, update, partnerID)
			}
		}

		if update.Message == nil {
			continue
		}

		// Check if the user has started the bot
		if update.Message.Text == "/start" {
			startBot(bot, update)
			continue
		}

		// Handle user responses to questions
		handleUserResponse(bot, update)
	}
}

// Add the following function to handle accepting a follow request
func handleAcceptFollow(bot *tgbotapi.BotAPI, update tgbotapi.Update, partnerID int64) {
	var existUser User
	if err := db.Where("telegram_id = ?", update.CallbackQuery.Message.Chat.ID).First(&existUser).Error; err != nil {
		fmt.Println("user not exist")
		return
	}

	// Update the follow request status in the database
	result := db.Model(&FollowRequest{}).
		Where("requester_id = ? AND target_id = ? AND accepted = ?", partnerID, existUser.TelegramID, false).
		Update("accepted", true)
	if result.RowsAffected == 0 {
		sendMessage(bot, existUser.TelegramID, "No follow request found to Accept.", backToHomeMenuKeyboard)
		return
	}

	// Send a message to the requester that the follow request is accepted
	var messageText string
	if existUser.Username != "" {
		// If the user has a username, include it in the message
		messageText = fmt.Sprintf("%s has accepted your follow request! 🎉\nAccepted username: @%s", existUser.Name, existUser.Username)
	} else {
		// If the user doesn't have a username, include a link with their Telegram ID
		messageText = fmt.Sprintf("%s has accepted your follow request! 🎉\nAccepted Mobile Number: %s", existUser.Name, existUser.MobileNumber)
	}
	sendMessage(bot, partnerID, messageText, backToHomeMenuKeyboard)

	// Instruct the requester to start a conversation
	var messageText2 string
	var existUser2 User
	if err := db.Where("telegram_id = ?", partnerID).First(&existUser2).Error; err != nil {
		fmt.Println("existUser2 not exist")
		return
	}
	if existUser2.Username != "" {
		// If the user has a username, include it in the message
		messageText2 = fmt.Sprintf("You have accepted the follow request \n username: @%s", existUser2.Username)
	} else {
		// If the user doesn't have a username, include a link with their Telegram ID
		messageText2 = fmt.Sprintf("You have accepted the follow request \n Mobile Number: %s: ", existUser2.MobileNumber)
	}
	sendMessage(bot, existUser.TelegramID, messageText2, backToHomeMenuKeyboard)
}

// Add the following function to handle declining a follow request
func handleDeclineFollow(bot *tgbotapi.BotAPI, update tgbotapi.Update, partnerID int64) {
	var existUser User
	if err := db.Where("telegram_id = ?", update.CallbackQuery.Message.Chat.ID).First(&existUser).Error; err != nil {
		fmt.Println("user not exist")
		return
	}

	// Delete the follow request from the database
	result := db.Where("requester_id = ? AND target_id = ? AND accepted = ?", partnerID, existUser.TelegramID, false).Delete(&FollowRequest{})
	if result.RowsAffected == 0 {
		sendMessage(bot, existUser.TelegramID, "No follow request found to delete.", backToHomeMenuKeyboard)
		return
	}

	// Send a message to the requester that the follow request is declined
	sendMessage(bot, partnerID, fmt.Sprintf("%s has declined your follow request. 😔", existUser.Name), backToHomeMenuKeyboard)

	// Send a message to the partner that the follow request is declined
	sendMessage(bot, existUser.TelegramID, fmt.Sprintf("You have declined the follow request."), backToHomeMenuKeyboard)
}

// startBot handles the initial interaction when the user starts the bot
func startBot(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Check if the user already exists in the database
	userID := int(update.Message.Chat.ID)
	var existingUser User
	if err := db.Where("telegram_id = ? AND name IS NOT NULL AND mobile_number IS NOT NULL AND name != '' AND mobile_number != ''", userID).First(&existingUser).Error; err == nil {
		// User already exists, display a message or a button
		sendExistingUserMessage(bot, update.Message.Chat.ID, &existingUser)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome to the English Partner Go Bot! 🇬🇧👥\n\n"+
		"سلام از آشنایی با شما خوشحالم. این ربات به شما کمک میکند تا پارتنر هم سطح خود برای تمرین زبان انگلیسی پیدا کنید. مراحل ثبت نام را انجام دهید و پروفایل خود را بسازید و سپس شروع کنید. شماره موبایل شما محفوظ و غیر قابل نمایش خواهد ماند و تنها آی دی تلگرام شما و عکس پروفایلی که اپلود میکنید به دیگران نمایش داده خواهد شد.")

	// Create a new user record or retrieve the existing record
	var user User
	db.FirstOrCreate(&user, User{TelegramID: int64(update.Message.Chat.ID)})
	user.CurrentQuestion = QuestionName // Start with the first question
	db.Save(&user)

	bot.Send(msg)
	askQuestion(bot, update.Message.Chat.ID, &user)
}

// sendExistingUserMessage sends a message or button to an existing user
func sendExistingUserMessage(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	sendReplyBackMessageFeatures(bot, int64(user.TelegramID), user)
}

// sendReplyBackMessageFeatures sends a welcome back message to existing users
func sendReplyBackMessageFeatures(bot *tgbotapi.BotAPI, chatID int64, user *User, dynamicMessageArgs ...interface{}) {
	// Customize this message based on your requirements
	var welcomeMessage string

	// Check if dynamic message arguments are provided
	if len(dynamicMessageArgs) > 0 {
		welcomeMessage = fmt.Sprintf("%s .", dynamicMessageArgs[0])
	} else {
		welcomeMessage = "🤗 Welcome back! 👋"
	}

	msg := tgbotapi.NewMessage(chatID, welcomeMessage)
	msg.ReplyMarkup = mainKeyboard
	bot.Send(msg)
}

// askQuestion sends a question to the user
func askQuestion(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	switch user.CurrentQuestion {
	case QuestionName:
		sendMessage(bot, chatID, "What's your name?")
	case QuestionMobileNumber:
		sendMessage(bot, chatID, "What's your mobile number?", iDoNotWantToEnterMobileMenuKeyboard)
	case QuestionEnglishLevel:
		sendMessage(bot, chatID, "What's your English level?", englishLevelKeyboard)
	case QuestionProfilePhoto:
		sendPhotoQuestion(bot, chatID)
	// case QuestionLocation:
	// 	sendMessage(bot, chatID, "Get your location (optional).")
	case QuestionGender:
		sendMessage(bot, chatID, "What is your gender?", selectGenderKeyboard)
	case QuestionDone:
		processUserAnswers(bot, chatID, user)
	}
}

func sendPhotoQuestion(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Upload your profile photo.")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	bot.Send(msg)
}

// sendMessage sends a message to the user
func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string, args ...interface{}) {

	msg := tgbotapi.NewMessage(chatID, text)
	if len(args) > 0 {
		msg.ReplyMarkup = args[0]
	} else {
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}

	bot.Send(msg)
}

// handleUserResponse handles user responses to questions
func handleUserResponse(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Retrieve user data from the database
	var user User
	db.FirstOrCreate(&user, User{TelegramID: int64(update.Message.Chat.ID)})

	// Process the user's response
	switch update.Message.Text {
	case "Next Question":
		processNextQuestion(bot, update.Message.Chat.ID, &user)
	case "🧑‍💼 Show Profile":
		// Display details for existing users
		showUserDetails(bot, update.Message.Chat.ID, &user)
	case "🧑‍💼🛠️ Edit Profile":
		sendMessage(bot, update.Message.Chat.ID, "Choose one of the options below:", editProfileMenuKeyboard)
	case "👤 Edit Name":
		setCurrentEditProfileQuestion(&user, "name")
		sendMessage(bot, update.Message.Chat.ID, "Please Type Your Name:", editProfileMenuKeyboard)
	case "🗣️🌍 Edit English Level":
		setCurrentEditProfileQuestion(&user, "english_level")
		sendMessage(bot, update.Message.Chat.ID, "Please Select Your English Level:", editEnglishLevelKeyboard)
	case "👫 Edit Gender":
		setCurrentEditProfileQuestion(&user, "gender")
		sendMessage(bot, update.Message.Chat.ID, "Please Select Your Gender:", editGenderKeyboard)
	case "🖼️ Edit Profile Photo":
		setCurrentEditProfileQuestion(&user, "profile_photo")
		sendMessage(bot, update.Message.Chat.ID, "Please Upload Your Profile Photo:", editProfileMenuKeyboard)
	case "🤜🤛👥 Find Partner":
		// Start the process of finding a partner
		handleFindPartner(bot, update.Message.Chat.ID, &user)
	case "➡️ Next Partner":
		// Start the process of Next Partner
		handleNextPartner(bot, update.Message.Chat.ID, &user)
	case "✅ Follow Partner":
		// Start the process of Follow Partner
		handleFollowRequest(bot, update, &user)
	case "🏠 Back To Home Menu":
		// Go To Home Menu
		startBot(bot, update)
	default:
		// Process responses to filter questions during finding a partner
		if user.CurrentFindPartnerQuestion == 1000 {
			handleEnglishLevelFilter(bot, update, &user)
		} else if user.CurrentFindPartnerQuestion == 1001 {
			handleGenderFilter(bot, update, &user)
		} else {
			// Process responses to other questions
			processUserAnswer(bot, update, &user)
		}
	}
}

// handle follow request function
func handleFollowRequest(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	partners, err := getPartnersFromCache(user.TelegramID)
	if err != nil {
		// Handle the error
		log.Println("Error retrieving partners from cache for handle follow request function:", err)
		return
	}
	// Get the user ID of the partner to follow
	partnerID := partners[user.CurrentNumberInPartnerList].TelegramID

	// Check if a follow request already exists
	if !isFollowRequestExists(user.TelegramID, partnerID) {
		// Create a new follow request
		followRequest := FollowRequest{
			RequesterID: user.TelegramID,
			TargetID:    partnerID,
			Accepted:    false, // You can set this to true if you want to automatically accept follow requests
		}
		db.Create(&followRequest)

		// Send a follow request message to the partner
		sendFollowRequestMessage(bot, partnerID, user, user.TelegramID)

		// Notify the user that the follow request is sent
		sendMessage(bot, update.Message.Chat.ID, "Your follow request has been sent!", backToHomeMenuKeyboard)
	} else {
		// Notify the user that a follow request already exists
		sendMessage(bot, update.Message.Chat.ID, "You have already sent a follow request to this partner.", backToHomeMenuKeyboard)
	}
}

func sendFollowRequestMessage(bot *tgbotapi.BotAPI, partnerID int64, user *User, requesterID int64) {
	// Customize the follow request message
	messageText := fmt.Sprintf("%s is requesting to follow you. ✅ Accept or ❌ Decline? \nEnglish Level: %s\n", user.Name, user.EnglishLevel)

	// Create a keyboard with accept and decline buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Accept", fmt.Sprintf("accept_follow:%d", requesterID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Decline", fmt.Sprintf("decline_follow:%d", requesterID)),
		),
	)

	if user.MediaID != 0 {
		// Get the media record
		var media Media
		if err := db.First(&media, user.MediaID).Error; err != nil {
			log.Println("Error getting media record for follow requester user:", err)
			return
		}

		// send Profile Detail With Caption
		photo := tgbotapi.NewPhotoUpload(partnerID, media.Filename)
		photo.Caption = messageText
		photo.ReplyMarkup = keyboard
		bot.Send(photo)
	}
}

// Add the following function to check if a follow request already exists
func isFollowRequestExists(requesterID, targetID int64) bool {
	var count int
	db.Model(&FollowRequest{}).Where("requester_id = ? AND target_id = ?", requesterID, targetID).Count(&count)
	return count > 0
}

// handle Next Partner Function
func handleNextPartner(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	var currentPositionInCache int
	if user.CurrentNumberInPartnerList > 0 {
		currentPositionInCache = user.CurrentNumberInPartnerList + 1
	} else {
		currentPositionInCache = 1
	}
	partners, err := getPartnersFromCache(user.TelegramID)
	if err != nil {
		// Handle the error
		log.Println("Error retrieving partners from cache:", err)
		return
	}

	// Use the partners data as needed
	partnersLength := len(partners)
	if partnersLength > currentPositionInCache {
		showPartnerDetail(bot, chatID, user, partners, currentPositionInCache)
		user.CurrentNumberInPartnerList = currentPositionInCache
		db.Save(user)
	} else {
		sendErrorMessage(bot, chatID, "dont exist another partner for you.")
	}
}

// showUserDetails displays user details, including the image, for existing users
func showUserDetails(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	// Customize this message based on the details you want to show
	profileDetailsText := fmt.Sprintf("🧑‍💼 User Profile Details:\nName: %s\nMobile Number: %s\nEnglish Level: %s\nGender: %s",
		user.Name, user.MobileNumber, user.EnglishLevel, user.Gender)

	// Check if the user has a profile photo
	if user.MediaID != 0 {
		// Get the media record
		var media Media
		if err := db.First(&media, user.MediaID).Error; err != nil {
			log.Println("Error getting media record:", err)
			return
		}

		// send Profile Detail With Caption
		photo := tgbotapi.NewPhotoUpload(chatID, media.Filename)
		photo.Caption = profileDetailsText
		bot.Send(photo)
	}
}

// handle edit profile, for existing users
func handleEditProfileName(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	if reflect.TypeOf(update.Message.Text).Kind() == reflect.String {
		user.Name = update.Message.Text
		db.Save(user)
		setCurrentEditProfileQuestion(user, "empty")
		sendMessage(bot, update.Message.Chat.ID, "Your name has been edited successfully", editProfileMenuKeyboard)
	} else {
		sendMessage(bot, update.Message.Chat.ID, "Please type your name", editProfileMenuKeyboard)
	}
}

func handleEditProfileEnglishLevel(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	if update.Message.Text == "Beginner" || update.Message.Text == "Intermediate" || update.Message.Text == "Advanced" {
		user.EnglishLevel = update.Message.Text
		db.Save(user)
		setCurrentEditProfileQuestion(user, "empty")
		sendMessage(bot, update.Message.Chat.ID, "Your English Level has been edited successfully", editProfileMenuKeyboard)
	} else {
		sendMessage(bot, update.Message.Chat.ID, "Please Select Your English Level", editEnglishLevelKeyboard)
	}
}

func handleEditProfileGender(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	if update.Message.Text == "👨 Male" {
		user.Gender = "male"
	} else if update.Message.Text == "👩 Female" {
		user.Gender = "female"
	} else {
		sendMessage(bot, update.Message.Chat.ID, "Please Select Your Gender", editGenderKeyboard)
		return
	}

	db.Save(user)
	setCurrentEditProfileQuestion(user, "empty")
	sendMessage(bot, update.Message.Chat.ID, "Your Gender has been edited successfully", editProfileMenuKeyboard)
}

func handlePhotoUpload(bot *tgbotapi.BotAPI, message *tgbotapi.Message) uint {
	// Check if the user uploaded a photo
	if message.Photo != nil && len(*message.Photo) > 0 {
		photo := getHighestQualityPhoto(*message.Photo)
		// Assuming you want to store the first photo in the Media table
		fileID := photo.FileID
		mediaID := savePhotoToMediaTable(bot, fileID, message.Chat.ID)

		// log.Printf("User record updated: ID %d, MediaID %d\n", message.Chat.ID, mediaID)
		return mediaID
	} else {
		// Handle the case where the user did not upload a photo
		sendErrorMessage(bot, message.Chat.ID, "Please upload a photo.")
		return 0
	}
}

func getHighestQualityPhoto(photos []tgbotapi.PhotoSize) tgbotapi.PhotoSize {
	highestQualityIndex := 0
	highestQualitySize := 0

	for i, photo := range photos {
		size := photo.FileSize
		if size > highestQualitySize {
			highestQualitySize = size
			highestQualityIndex = i
		}
	}

	return photos[highestQualityIndex]
}

func savePhotoToMediaTable(bot *tgbotapi.BotAPI, fileID string, userTelegramID int64) uint {
	// Get the photo file path from Telegram
	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Println("Error getting file:", err)
	}

	// Download the photo file using a standard HTTP request
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", bot.Token, file.FilePath)
	resp, err := http.Get(fileURL)
	if err != nil {
		log.Println("Error downloading file:", err)
	}
	defer resp.Body.Close()

	// Save the file to the storage directory with the user's Telegram ID as the filename
	filename := fmt.Sprintf("storage/%d%s", userTelegramID, filepath.Ext(file.FilePath))
	err = saveFile(filename, resp.Body)
	if err != nil {
		log.Println("Error saving file:", err)
	}

	// Save the file information to the Media table
	var media Media
	media.Filename = filename
	if err := db.Create(&media).Error; err != nil {
		log.Println("Error creating media record:", err)
		// return 0
	}
	// log.Printf("Media record created: ID %d, Filename %s\n", media.ID, media.Filename)
	return media.ID
}

func saveFile(filename string, body io.Reader) error {
	// Ensure the "storage" directory exists
	err := os.MkdirAll("storage", os.ModePerm)
	if err != nil {
		return err
	}

	// Save the file to the specified path
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, body)
	return err
}

var userMutex sync.Mutex

// processUserAnswer processes the user's answer and updates the user struct
func processUserAnswer(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	// Lock the mutex to ensure exclusive access to the user object
	userMutex.Lock()
	defer userMutex.Unlock()

	// store username
	user.Username = update.Message.From.UserName

	// Save the user's answer to the current question
	switch user.CurrentQuestion {
	case QuestionName:
		user.Name = update.Message.Text
	case QuestionMobileNumber:
		if update.Message.Text == "⏭️ I do not want to enter mobile number" {
			user.MobileNumber = "empty"
		} else {
			checkValidMobile := isValidMobileNumber(update.Message.Text)
			if checkValidMobile != "error" {
				user.MobileNumber = checkValidMobile
			} else {
				sendErrorMessage(bot, update.Message.Chat.ID, "Invalid mobile number. Please enter a valid 11-digit mobile number.")
				return
			} // End if
		} // End if
	case QuestionEnglishLevel:
		if isValidEnglishLevel(update.Message.Text) {
			user.EnglishLevel = update.Message.Text
		} else {
			sendErrorMessage(bot, update.Message.Chat.ID, "Invalid English level. Please select from Beginner, Intermediate, or Advanced.")
			return
		}
	case QuestionProfilePhoto:
		// Check if the user uploaded a photo
		if update.Message.Photo != nil && len(*update.Message.Photo) > 0 {
			// Assuming the user sent a photo, you can handle file/photo uploads here
			handleExistingUser(bot, user)
			mediaID := handlePhotoUpload(bot, update.Message)
			user.MediaID = mediaID
		} else {
			// Handle the case where the user did not upload a photo
			sendErrorMessage(bot, update.Message.Chat.ID, "Please Upload a Your Profile Photo.")
			return
		}
	case QuestionGender:
		SelectedGender := validSelectedGender(update.Message.Text)
		if SelectedGender == "male" || SelectedGender == "female" {
			user.Gender = SelectedGender
		} else {
			sendErrorMessage(bot, update.Message.Chat.ID, "Invalid Gender. Please select from Male or Female.")
			return
		}

	// case QuestionLocation:
	// 	// Assuming the user sent a location, you can handle location updates here
	// 	if update.Message.Location != nil {
	// 		latitude := update.Message.Location.Latitude
	// 		longitude := update.Message.Location.Longitude

	// 		storeLocationInDatabase(user, latitude, longitude)
	// 	} // End if
	default:
		if user.CurrentEditProfileQuestion != "empty" {
			switch user.CurrentEditProfileQuestion {
			case "name":
				handleEditProfileName(bot, update, user)
			case "english_level":
				handleEditProfileEnglishLevel(bot, update, user)
			case "gender":
				handleEditProfileGender(bot, update, user)
			case "profile_photo":
				// Check if the user uploaded a photo
				if update.Message.Photo != nil && len(*update.Message.Photo) > 0 {
					handleExistingUser(bot, user)
					mediaID := handlePhotoUpload(bot, update.Message)
					user.MediaID = mediaID
					db.Save(user)
					setCurrentEditProfileQuestion(user, "empty")
					sendMessage(bot, update.Message.Chat.ID, "Your Profile Photo has been updated successfully", editProfileMenuKeyboard)
				} else {
					sendErrorMessage(bot, update.Message.Chat.ID, "Please Upload a Your Profile Photo.")
					return
				} // End if
			}
			// End switch edit profile
		} else {
			return
		}
	} // End switch

	// Move to the next question or finish the registration
	processNextQuestion(bot, update.Message.Chat.ID, user)
}

func validSelectedGender(gender string) string {
	if gender == "👨 Male" {
		return "male"
	}
	if gender == "👩 Female" {
		return "female"
	}
	if gender == "🤷‍♂️ Does Not Matter" {
		return "no matter"
	}
	return "error"
}

func storeLocationInDatabase(user *User, latitude, longitude float64) {
	// Update user's latitude and longitude
	user.Latitude = latitude
	user.Longitude = longitude

	// Save the updated user
	if err := db.Save(&user).Error; err != nil {
		log.Println("Error saving location data user:", err)
		return
	}
}

// handleExistingUser handles the case when the user already exists in the database
func handleExistingUser(bot *tgbotapi.BotAPI, user *User) {
	// Check if the user has a profile photo in the Media table
	if user.MediaID != 0 {
		// Get the media record
		var media Media
		if err := db.First(&media, user.MediaID).Error; err != nil {
			log.Println("Error getting media record:", err)
			return
		}

		// Remove the file from the storage directory
		err := os.Remove(media.Filename)
		if err != nil {
			log.Println("Error removing file from storage:", err)
		}

		// Remove the media record from the database
		if err := db.Delete(&media).Error; err != nil {
			log.Println("Error deleting media record:", err)
			return
		}

		// Reset the user's media ID
		user.MediaID = 0

		// Save the updated user record
		if err := db.Save(user).Error; err != nil {
			log.Println("Error updating user record:", err)
			return
		}
	} // End if
}

// processNextQuestion moves the user to the next question or finishes the registration
func processNextQuestion(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	// Move to the next question
	user.CurrentQuestion++
	db.Save(user)

	// Ask the next question or finish the registration
	askQuestion(bot, chatID, user)
}

// processUserAnswers processes the user's answers after all questions are answered
func processUserAnswers(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	// You need to implement logic to store user's answers in the database
	// For demonstration purposes, we'll print the answers to the console

	// Implement your database storage logic here
	// Example: Save the user data to the database
	if err := db.Save(user).Error; err != nil {
		sendErrorMessage(bot, chatID, "Failed to store user data. Please try again.")
		return
	}

	// Show a success message to the user
	successMessage := "Thank you for completing the registration! You are now a registered user."
	sendReplyBackMessageFeatures(bot, int64(user.TelegramID), user, successMessage)
}

// isValidMobileNumber checks if the provided string is a valid 11-digit mobile number
func isValidMobileNumber(mobileNumber string) string {
	// Remove spaces, "-" and "_"
	mobileNumber = strings.ReplaceAll(mobileNumber, " ", "")
	mobileNumber = strings.ReplaceAll(mobileNumber, "-", "")
	mobileNumber = strings.ReplaceAll(mobileNumber, "_", "")

	// Check if the resulting string is 11 digits and contains only numeric characters
	if len(mobileNumber) == 11 && isNumeric(mobileNumber) {
		return mobileNumber
	} else {
		return "error"
	}
}

// isNumeric checks if a given string contains only numeric characters
func isNumeric(str string) bool {
	for _, char := range str {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

// isValidEnglishLevel checks if the provided string is a valid English level
func isValidEnglishLevel(englishLevel string) bool {
	// Implement your English level validation logic here
	if englishLevel == "Beginner" || englishLevel == "Intermediate" || englishLevel == "Advanced" {
		return englishLevel != ""
	} else {
		return false
	}
}

// sendErrorMessage sends an error message to the user
func sendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	bot.Send(msg)
}

// *** Find Partner Functions ***
// handleFindPartner initiates the process of finding a partner
func handleFindPartner(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	// Ask the first filter question (English level)
	setCurrentFindPartnerQuestion(user)
	sendMessage(bot, chatID, "What's the preferred English level of your potential partner?", englishLevelKeyboard)
}

func setCurrentFindPartnerQuestion(user *User, number ...int) {
	if len(number) > 0 {
		user.CurrentFindPartnerQuestion = number[0]
	} else {
		user.CurrentFindPartnerQuestion = 1000
	}
	db.Save(user)
}

func setCurrentEditProfileQuestion(user *User, question string) {
	if question == "empty" {
		user.CurrentEditProfileQuestion = "empty"
	} else {
		user.CurrentEditProfileQuestion = question
	}
	db.Save(user)
}

// handleEnglishLevelFilter processes the user's English level filter response
func handleEnglishLevelFilter(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	switch update.Message.Text {
	case "Beginner", "Intermediate", "Advanced":
		user.CurrentFindPartnerQuestion = 1001
		user.LastSelectedEnglishLevel = update.Message.Text
		db.Save(user)
		// Ask the next filter question (gender)
		sendMessage(bot, update.Message.Chat.ID, "What's the preferred gender of your potential partner?", selectGenderFilterKeyboard)
	default:
		sendErrorMessage(bot, update.Message.Chat.ID, "Invalid English level option. Please select from Beginner, Intermediate, or Advanced.")
	}
}

// handleGenderFilter processes the user's gender filter response
func handleGenderFilter(bot *tgbotapi.BotAPI, update tgbotapi.Update, user *User) {
	switch update.Message.Text {
	case "👨 Male", "👩 Female", "🤷‍♂️ Does Not Matter":
		// Move to the next question in the context of finding a partner
		setCurrentFindPartnerQuestion(user)
		checkValidGender := validSelectedGender(update.Message.Text)
		if checkValidGender == "error" {
			sendErrorMessage(bot, update.Message.Chat.ID, "Invalid gender option. Please select from Male or Female.")
		}
		user.LastSelectedGender = checkValidGender
		db.Save(user)

		processFindPartnerAnswers(bot, update.Message.Chat.ID, user)
	default:
		sendErrorMessage(bot, update.Message.Chat.ID, "Invalid gender option. Please select from Male or Female.")
	}
}

// processFindPartnerAnswers processes the user's answers after all questions are answered in the context of finding a partner
func processFindPartnerAnswers(bot *tgbotapi.BotAPI, chatID int64, user *User) {
	// Get partners based on filters (English level and gender)
	partners := getMatchingPartners(user.LastSelectedEnglishLevel, user.LastSelectedGender, user.TelegramID)

	// Cache partners in Redis
	cachePartners(partners, user.TelegramID)

	// Display the first partner to the user
	if len(partners) > 0 {

		// Update LastFindPartnerTime to the current timestamp
		if has24HoursPassed(user.LastFindPartnerTime) == true {
			user.LastFindPartnerTime = time.Now()
			user.CountWatchPartnerLimit = 0
			db.Save(user)
		}

		showPartnerDetail(bot, chatID, user, partners, 0)
	} else {
		// Inform the user that no matching partners were found
		sendMessage(bot, chatID, "No matching partners found. Try adjusting your preferences.", mainKeyboard)
	}

	// Reset the user's Find Partner question state
	setCurrentFindPartnerQuestion(user, 1002)
}

// has24HoursPassed checks if 24 hours have passed since the stored time
func has24HoursPassed(lastTime time.Time) bool {
	// Calculate the time difference
	timeDifference := time.Since(lastTime)
	// Calculate the difference between the current time and the stored time
	hoursPassed := int(timeDifference.Hours())

	return hoursPassed >= 24
}

// Show Partner Detail To user
func showPartnerDetail(bot *tgbotapi.BotAPI, chatID int64, user *User, partners []*User, partnerKeyToShow int) {
	// check user time & count limit for watch partner per day
	if has24HoursPassed(user.LastFindPartnerTime) == false && user.CountWatchPartnerLimit >= 20 {
		sendMessage(bot, chatID, "🔒⏰ You need to wait 24 hours before finding the next partner.", backToHomeMenuKeyboard)
		return
	}

	// Customize this message based on the details you want to show
	partnerDetailsText := fmt.Sprintf("👥 Partner Details:\nName: %s\nEnglish Level: %s\n",
		partners[partnerKeyToShow].Name, partners[partnerKeyToShow].EnglishLevel)

	// set current number in partner list to 0
	user.CurrentNumberInPartnerList = 0

	// Add Count watch partner limit
	user.CountWatchPartnerLimit = user.CountWatchPartnerLimit + 1
	db.Save(user)

	// Add Seen User ID in WatchList model
	newWatch := WatchList{
		UserID:  chatID,
		WatchID: partners[partnerKeyToShow].TelegramID,
	}
	db.Create(&newWatch)

	// Check if the user has a profile photo
	if partners[partnerKeyToShow].MediaID != 0 {
		// Get the media record
		var media Media
		if err := db.First(&media, partners[partnerKeyToShow].MediaID).Error; err != nil {
			log.Println("Error getting partner media record:", err)
			return
		}

		// send Profile Detail With Caption
		photo := tgbotapi.NewPhotoUpload(chatID, media.Filename)
		photo.Caption = partnerDetailsText
		bot.Send(photo)
	} else {
		// Show partner details to the user
		sendMessage(bot, chatID, partnerDetailsText, selectNextOrAcceptPartnerKeyboard)
	}
	// show Accept or Next Menu to user
	sendMessage(bot, chatID, "Please ✅ Follow or Watch ➡️ Next Partner...", selectNextOrAcceptPartnerKeyboard)
}

// cachePartners caches the list of partners in Redis
func cachePartners(partners []*User, telegramID int64) {
	// Convert partners to JSON
	partnersJSON, err := json.Marshal(partners)
	if err != nil {
		log.Println("Error marshaling partners to JSON:", err)
		return
	}

	telegramIDStr := strconv.FormatInt(telegramID, 10)
	redisKey := "partners:" + telegramIDStr

	// Store partners in Redis with an expiration time (e.g., 24 hours)
	err = redisClient.Set(context.Background(), redisKey, partnersJSON, 12*time.Hour).Err()
	if err != nil {
		log.Println("Error caching partners in Redis:", err)
	}
}

// getPartnersFromCache retrieves partner data from the Redis cache based on telegramID and keyNumber
func getPartnersFromCache(telegramID int64) ([]*User, error) {
	telegramIDStr := strconv.FormatInt(telegramID, 10)
	redisKey := fmt.Sprintf("partners:%s", telegramIDStr)

	partnersJSON, err := redisClient.Get(context.Background(), redisKey).Result()
	if err != nil {
		log.Println("Error getting partners from Redis:", err)
		return nil, err
	}

	// Unmarshal the JSON data into []*User
	var partners []*User
	if err := json.Unmarshal([]byte(partnersJSON), &partners); err != nil {
		log.Println("Error unmarshaling partners from JSON:", err)
		return nil, err
	}

	return partners, nil
}

// getMatchingPartners retrieves partners from the database based on English level and gender filters
func getMatchingPartners(englishLevel, gender string, telegramID int64) []*User {
	// Implement your logic to query the database for matching partners
	var matchingPartners []*User

	// Get the watch IDs for the given user
	watchIDs := getWatchIDs(telegramID)

	if gender == "no matter" {
		query := db.Where("english_level = ? AND telegram_id != ?", englishLevel, telegramID).Limit(10)

		if len(watchIDs) > 0 {
			query = query.Not("telegram_id IN (?)", watchIDs)
		}

		if err := query.Find(&matchingPartners).Error; err != nil {
			log.Println("Error querying database for matching partners:", err)
		}
	} else {
		query := db.Where("english_level = ? AND gender = ? AND telegram_id != ?", englishLevel, gender, telegramID).Limit(10)

		if len(watchIDs) > 0 {
			query = query.Not("telegram_id IN (?)", watchIDs)
		}

		if err := query.Find(&matchingPartners).Error; err != nil {
			log.Println("Error querying database for matching partners:", err)
		}
	}

	return matchingPartners
}

// getWatchIDs retrieves WatchID values for a given user from the WatchList model
func getWatchIDs(telegramID int64) []int64 {
	var watchIDs []int64

	// Query the WatchList model to get WatchID values for the given user
	if err := db.Model(&WatchList{}).Where("user_id = ?", telegramID).Pluck("watch_id", &watchIDs).Error; err != nil {
		log.Println("Error querying database for watch IDs:", err)
	}

	return watchIDs
}
