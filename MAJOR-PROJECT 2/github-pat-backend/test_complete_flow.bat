@echo off
REM Complete Flow Test Script for Windows
REM This script tests the entire backend flow

echo ========================================
echo Aegios Backend Complete Flow Test
echo ========================================
echo.

REM Set variables
set BACKEND_URL=http://localhost:8080
set EMAIL=test@example.com
set PASSWORD=Test123!
set GITHUB_USERNAME=YOUR_GITHUB_USERNAME
set GITHUB_PAT=YOUR_GITHUB_PAT

echo Step 1: Testing Health Check...
curl -s %BACKEND_URL%/health
echo.
echo.

echo Step 2: Testing Signup...
curl -X POST %BACKEND_URL%/authentication/signup ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"testuser\",\"email\":\"%EMAIL%\",\"password\":\"%PASSWORD%\",\"github_username\":\"%GITHUB_USERNAME%\",\"github_pat\":\"%GITHUB_PAT%\"}"
echo.
echo.

echo Step 3: Testing Login...
echo Saving session token...
curl -X POST %BACKEND_URL%/authentication/login ^
  -H "Content-Type: application/json" ^
  -d "{\"email\":\"%EMAIL%\",\"password\":\"%PASSWORD%\"}" > login_response.json
echo.
echo Login response saved to login_response.json
echo Please extract session_token and set it in SESSION_TOKEN variable
echo.

pause

echo Step 4: Testing Fetch Data...
echo Enter your session token:
set /p SESSION_TOKEN=
curl -X POST %BACKEND_URL%/fetching-service/fetch-data ^
  -H "Content-Type: application/json" ^
  -d "{\"session_token\":\"%SESSION_TOKEN%\"}"
echo.
echo.

echo Step 5: Testing Posture Endpoint...
curl -X POST %BACKEND_URL%/security-service/k8s-posture ^
  -H "Content-Type: application/json" ^
  -d "{\"session_token\":\"%SESSION_TOKEN%\"}"
echo.
echo.

echo Step 6: Testing Score Endpoint...
curl -X POST %BACKEND_URL%/security-service/k8s-score ^
  -H "Content-Type: application/json" ^
  -d "{\"session_token\":\"%SESSION_TOKEN%\"}"
echo.
echo.

echo ========================================
echo Test Complete!
echo ========================================
echo.
echo Next Steps:
echo 1. Check if all endpoints returned success: true
echo 2. Verify database has data: SELECT COUNT(*) FROM kubernetes_resource;
echo 3. Test frontend at http://localhost:5173
echo.

pause
