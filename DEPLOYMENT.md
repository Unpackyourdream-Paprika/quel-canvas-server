# Deployment Guide for Canvas Collaboration Server

## Ready for Deployment

The Go WebSocket server is now ready for deployment to Render.com. All necessary files have been prepared:

- ✅ `main.go` - Complete WebSocket server with session management
- ✅ `go.mod` - Go dependencies
- ✅ `render.yaml` - Render.com deployment configuration
- ✅ `.gitignore` - Git ignore rules
- ✅ `README.md` - API documentation
- ✅ Git repository initialized and committed

## Steps to Deploy to Render.com

### 1. Create GitHub Repository

```bash
# You need to create a GitHub repository and push this code
git remote add origin https://github.com/YOUR_USERNAME/quel-canvas-server.git
git branch -M main
git push -u origin main
```

### 2. Deploy on Render.com

1. Go to [Render.com](https://render.com) and sign in
2. Click "New +" → "Web Service"
3. Connect your GitHub repository `quel-canvas-server`
4. Render will automatically detect the `render.yaml` configuration
5. Click "Deploy"

The deployment configuration in `render.yaml` will:
- Build the Go application
- Set up the PORT environment variable automatically
- Start the server

### 3. Update Frontend Configuration

Once deployed, you'll get a URL like: `https://your-app-name.onrender.com`

Update your frontend environment variable:

```bash
# In /Users/asd/quel-light/.env.local
NEXT_PUBLIC_WEBSOCKET_SERVER_URL=wss://your-app-name.onrender.com
```

**Important**: Use `wss://` (secure WebSocket) for the production URL, not `ws://`.

### 4. Test the Deployment

1. Start your Next.js app: `npm run dev`
2. Navigate to a canvas page
3. Add `?collaboration=test-session&host=user1` to the URL
4. Open another browser/tab with `?collaboration=test-session&host=user2`
5. Test real-time collaboration features

## API Endpoints Available

- `GET /health` - Health check
- `GET /metrics` - Server metrics and session info
- `GET /session/{sessionId}` - Individual session info
- `WS /ws?session={sessionId}&user={userId}` - WebSocket connection
- `POST /admin/cleanup` - Force cleanup sessions

## Monitoring

Check server metrics at: `https://your-app-name.onrender.com/metrics`

## Local Testing

If you have Go installed locally, you can test the server:

```bash
go run main.go
```

Then open `test.html` in your browser to test WebSocket functionality.

## Production Notes

- The server automatically cleans up empty sessions every 5 minutes
- Sessions expire after 24 hours or 2 hours of inactivity
- All CORS origins are currently allowed (configure for production)
- WebSocket connections are monitored and logged