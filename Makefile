.PHONY: test lint build docker

# Fan out to both services. See go-service/ and py-service/ for the real targets.

test:
	$(MAKE) -C go-service test
	$(MAKE) -C py-service test

lint:
	$(MAKE) -C go-service lint
	$(MAKE) -C py-service lint

build:
	$(MAKE) -C go-service build
	$(MAKE) -C py-service install

docker:
	docker build -t ai-market-analyst/market-data:dev ./go-service
	docker build -t ai-market-analyst/ai-analyst:dev ./py-service
