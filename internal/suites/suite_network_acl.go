package suites

import (
	"time"
)

var networkACLSuiteName = "NetworkACL"

func init() {
	dockerEnvironment := NewDockerEnvironment([]string{
		"docker-compose.yml",
		"example/compose/authelia/docker-compose.backend.yml",
		"example/compose/authelia/docker-compose.frontend.yml",
		"example/compose/nginx/backend/docker-compose.yml",
		"example/compose/nginx/portal/docker-compose.yml",
		"example/compose/squid/docker-compose.yml",
		"example/compose/smtp/docker-compose.yml",
		// To debug headers
		"example/compose/httpbin/docker-compose.yml",
	})

	setup := func(suitePath string) error {
		if err := dockerEnvironment.Up(suitePath); err != nil {
			return err
		}

		return waitUntilAutheliaIsReady(dockerEnvironment)
	}

	teardown := func(suitePath string) error {
		return dockerEnvironment.Down(suitePath)
	}

	GlobalRegistry.Register(networkACLSuiteName, Suite{
		SetUp:           setup,
		SetUpTimeout:    5 * time.Minute,
		TestTimeout:     1 * time.Minute,
		TearDown:        teardown,
		TearDownTimeout: 2 * time.Minute,
		Description: `This suite has been created to test Authelia with basic feature in a non highly-available setup.
Authelia basically use an in-memory cache to store user sessions and persist data on disk instead
of using a remote database. Also, the user accounts are stored in file-based database.`,
	})
}
