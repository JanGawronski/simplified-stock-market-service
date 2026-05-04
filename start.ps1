param(
    [Parameter(Mandatory = $true)]
    [int]$Port
)

$env:HOST_PORT = "$Port"
docker compose up --build -d --remove-orphans
Write-Output "Stock market service is available at http://localhost:$Port"

