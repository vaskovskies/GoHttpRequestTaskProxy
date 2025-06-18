# Define the URL of the endpoint
$url = "http://admin:secret@localhost:8080/task"

# Send the GET request using Invoke-RestMethod
$response = Invoke-RestMethod -Uri $url -Method Delete

# Output the response
$response | ConvertTo-Json -Depth 10
