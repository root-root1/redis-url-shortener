package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/root-root1/redis_golang/database"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")
	r := database.CreateClient(0)
	defer r.Close()
	value, err := r.Get(database.Ctx, url).Result()

	if err != redis.Nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Data not fount"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Cannot Connect To db"})
	}

	rIncr := database.CreateClient(1)
	_ = rIncr.Incr(database.Ctx, "counter")

	return c.Redirect(value, 301)
}
