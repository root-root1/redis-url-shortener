package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/root-root1/redis_golang/database"
	"github.com/root-root1/redis_golang/helper"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"short"`
	Expiry      time.Duration `json:"expiry"`
}

type response struct {
	URL             string        `json:"url"`
	CustomShort     string        `json:"custom_short"`
	Expiry          time.Duration `json:"expiry"`
	XRateRemaining  int           `json: "rate_limit"`
	XRateLimitReset time.Duration `json:"rate_reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to parse"})
	}

	// implementation of rate limiter
	db := database.CreateClient(1)
	defer db.Close()

	val, err := db.Get(database.Ctx, c.IP()).Result()
	if err == redis.Nil {
		_ = db.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
	} else {
		// val, _ = db.Get(database.Ctx, c.IP()).Result()
		valInt, _ := strconv.Atoi(val)
		if valInt <= 0 {
			limit, _ := db.TTL(database.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":         "Rate Limit Execced",
				"rate_reset_in": limit / time.Nanosecond / time.Minute,
			})
		}
	}

	// checking for the url is url or not
	if !govalidator.IsURL(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Url"})
	}

	// checkung url domain
	if !helper.RemoveDomainError(body.URL) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "access Denied"})
	}
	body.URL = helper.EnforceHTTP(body.URL)

	var id string
	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	r := database.CreateClient(0)
	defer r.Close()

	val, _ = r.Get(database.Ctx, id).Result()

	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Custom Short already in user",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}
	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to Connect to Server"})
	}

	res := response{
		URL:             body.URL,
		CustomShort:     "",
		Expiry:          body.Expiry,
		XRateRemaining:  10,
		XRateLimitReset: 30,
	}

	db.Decr(database.Ctx, c.IP())

	val, _ = db.Get(database.Ctx, c.IP()).Result()
	res.XRateRemaining, _ = strconv.Atoi(val)

	ttl, _ := db.TTL(database.Ctx, c.IP()).Result()
	res.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	res.CustomShort = os.Getenv("DOMAIN") + "/" + id

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"sucess": "Done"})

}
