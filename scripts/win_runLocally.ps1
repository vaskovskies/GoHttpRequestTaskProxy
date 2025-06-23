# Set the environment variable temporarily
$env:DATABASE_URL = "postgresql://postgres:postgres@localhost:5432"

# Change to the parent directory
#Set-Location ..

# Run the Go application
go run cmd\go-http-request-task-proxy\main.go

# Optionally, clear the environment variable after running
Remove-Item Env:DATABASE_URL