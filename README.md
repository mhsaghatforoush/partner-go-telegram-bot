# 🤖 English Partner Finder Telegram Bot

[![GitHub stars](https://img.shields.io/github/stars/mhsaghatforoush/partner-go-telegram-bot.svg?style=flat-square)](https://github.com/mhsaghatforoush/partner-go-telegram-bot/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/mhsaghatforoush/partner-go-telegram-bot.svg?style=flat-square)](https://github.com/mhsaghatforoush/partner-go-telegram-bot/issues)
[![GitHub license](https://img.shields.io/github/license/mhsaghatforoush/partner-go-telegram-bot.svg?style=flat-square)](https://github.com/mhsaghatforoush/partner-go-telegram-bot/blob/main/LICENSE)

[👉👉👉Start a conversation with the English Partner Telegram Bot](https://t.me/partner_go_bot)

> ربات پارتنر یابی زبان انگلیسی, توسعه داده شده با زبان Go.

![Demo](demo.gif)

## Table of Contents

- [About](#about)
- [Features](#features)
- [Installation](#installation)

## About

The English Partner Finder Telegram Bot is designed to help users find language partners for practicing English conversation. The bot allows users to create profiles, specify their English proficiency level and gender preferences, and connect with other users who match their criteria.

## Features

- User authentication and profile creation
- English proficiency level selection (Beginner, Intermediate, Advanced)
- Gender preference selection
- Partner matching based on user profiles
- Follow request system for connecting with language partners

## Installation

1. **Clone the repository:**

   ```bash
   git clone https://github.com/mhsaghatforoush/partner-go-telegram-bot.git

2. **Set up environment variables by creating a .env file and set your telegram apitoken:**

   ```bash
   TELEGRAM_APITOKEN=YOUR_TOKEN

3. **Set up Postgres Database environment variables in gorm connection on main.go file:**

   ```bash
   gorm.Open("postgres", "host=localhost user=postgres dbname=your_db_name sslmode=disable password=your_db_password")

4. **Run the bot:**

   ```bash
   go run main.go

