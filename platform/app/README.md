# Solana Validator Risk Assessment System

A front-end and back-end separated system for monitoring and analyzing Solana validators, including performance metrics, stake distribution, and geographic visualization.

## Project Structure

```
platform/risk-assessment/
├── backend/           # Flask API server
│   ├── app.py        # Main API application
│   └── requirements.txt
└── frontend/          # React web interface
    ├── src/
    │   ├── components/
    │   │   ├── Dashboard.jsx      # Main validator list
    │   │   ├── ValidatorDetail.jsx # Detailed validator view
    │   │   └── ValidatorMap.jsx    # Geographic visualization
    │   ├── App.jsx
    │   ├── main.jsx
    │   └── index.css
    ├── index.html
    ├── package.json
    └── vite.config.js
```

## Features

### Backend API
- **GET /api/health** - Health check endpoint
- **GET /api/validators** - Get all validators for a specific epoch
- **GET /api/validators/:voteAccount** - Get detailed validator information
- **GET /api/epochs** - Get available epoch range
- **GET /api/stats** - Get overall statistics for an epoch

### Frontend Features
- **Dashboard**: View all validators with key metrics (stake, commission, location, APY)
- **Validator Detail**: Deep dive into individual validator performance and stake distribution
- **Geographic Map**: Visualize validator locations on an interactive world map
- Real-time data filtering by epoch
- Stake distribution analysis with Gini coefficient
- Top stakers visualization

## Setup Instructions

### Backend Setup

1. Navigate to backend directory:
```bash
cd platform/risk-assessment/backend
```

2. Install Python dependencies:
```bash
pip install -r requirements.txt
```

3. Make sure the parent platform directory has a valid `.env` file with:
```
EPOCHS_PATH=/path/to/epochs/data
SAM_PATH=/path/to/sam/data
```

4. Start the Flask server:
```bash
python app.py
```

The API will run on `http://localhost:5000`

### Frontend Setup

1. Navigate to frontend directory:
```bash
cd platform/risk-assessment/frontend
```

2. Install Node.js dependencies:
```bash
npm install
```

3. Start the development server:
```bash
npm run dev
```

The web interface will run on `http://localhost:3000`

## Usage

1. Start both backend and frontend servers
2. Open `http://localhost:3000` in your browser
3. Select an epoch to view validator data
4. Click on any validator name to see detailed information
5. Navigate to "Validator Map" to see geographic distribution

## Data Sources

The system reads validator data from:
- **validators.json**: Basic validator information
- **validator_info.json**: Validator metadata (name, website, etc.)
- **stakewiz.json**: IP location and APY data
- **marinade validators.json**: Additional location data
- **Individual stake files**: Detailed staking information per validator

## Technical Stack

**Backend:**
- Python 3.x
- Flask (Web framework)
- Flask-CORS (Cross-origin resource sharing)
- Pandas & NumPy (Data processing)

**Frontend:**
- React 18
- React Router (Navigation)
- Axios (HTTP client)
- Recharts (Data visualization)
- React-Leaflet (Map visualization)
- Vite (Build tool)

## API Examples

### Get all validators
```bash
curl "http://localhost:5000/api/validators?epoch=800&with_stake=false"
```

### Get validator details
```bash
curl "http://localhost:5000/api/validators/VOTE_ACCOUNT_PUBKEY?epoch=800"
```

### Get statistics
```bash
curl "http://localhost:5000/api/stats?epoch=800"
```

## Future Enhancements

- Risk scoring model integration
- Real-time transaction data (fast channel)
- Historical trend analysis
- Alert system for validator anomalies
- Advanced filtering cap and searchabilities
- Export functionality for reports
