.PHONY: build clean test coverage fmt lint update

build:
	@nu ./scripts/build.nu

clean:
	@nu ./scripts/clean.nu

test:
	@nu ./scripts/test.nu

coverage:
	@nu ./scripts/coverage.nu

fmt:
	@nu ./scripts/fmt.nu

lint:
	@nu ./scripts/lint.nu

update:
	@nu ./scripts/update.nu
