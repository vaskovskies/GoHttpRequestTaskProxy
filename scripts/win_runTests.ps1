# Set the environment variable temporarily
$env:DATABASE_URL = "postgresql://testpostgres:testpostgres@localhost:5432"

# Change to the parent directory
#Set-Location ..

# Run the Go application
go test

# Optionally, clear the environment variable after running
Remove-Item Env:DATABASE_URL