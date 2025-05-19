FROM grafana/k6:latest

WORKDIR /app
COPY script.js /app/script.js

ENTRYPOINT ["k6", "run", "--vus", "5", "--iterations", "10000", "script.js"]