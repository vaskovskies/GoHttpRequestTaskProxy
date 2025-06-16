# Define the RequestTask object
$requestTask = @{
    Method  = "POST"
    Url     = "https://httpbin.org/post"  # Change this to your test server URL
    Headers = @{
        "Content-Type" = "application/json"
        "Custom-Header" = "Value"  # Add any custom headers you need
    }
    Body    = "New Task"
}

# Convert the RequestTask object to JSON
$jsonBody = $requestTask | ConvertTo-Json -Depth 4

# Define your server URL
$serverUrl = "http://localhost:8080/task"  # Change this to your server's endpoint

# Send the POST request to your server
$response = Invoke-RestMethod -Uri $serverUrl -Method Post -Body $jsonBody -ContentType "application/json"

# Output the response from your server
Write-Host "Response from server:"
$response | ConvertTo-Json | Write-Host