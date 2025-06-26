# Set the environment variable temporarily
$env:DATABASE_URL = "postgresql://testpostgres:testpostgres@localhost:2345"

# Change to the parent directory
#Set-Location ..

# Run the Go application
go test ./cmd/go-http-request-task-proxy

# Optionally, clear the environment variable after running
Remove-Item Env:DATABASE_URL