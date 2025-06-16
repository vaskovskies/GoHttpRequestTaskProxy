# Set the environment variable temporarily
$env:DATABASE_URL = "host=go_db user=postgres password=postgres dbname=postgres sslmode=disable"

# Change to the parent directory
Set-Location ..

# Run the Go application
go run main.go

# Optionally, clear the environment variable after running
Remove-Item Env:DATABASE_URL