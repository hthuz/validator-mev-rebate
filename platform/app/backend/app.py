"""
Solana Validator Risk Assessment System - Backend API
"""
from flask import Flask, jsonify, request
from flask_cors import CORS
import sys
import os

# Add parent directory to path to import platform modules
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../../')))

# Load environment variables from platform directory
from dotenv import load_dotenv
platform_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), '../../'))
env_path = os.path.join(platform_dir, '.env')
load_dotenv(env_path)

from platform.stake_reader import Reader
from config import START_EPOCH, END_EPOCH

app = Flask(__name__)
CORS(app)  # Enable CORS for frontend communication

@app.route('/api/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({"status": "ok", "message": "Validator Risk Assessment API is running"})

@app.route('/api/validators', methods=['GET'])
def get_validators():
    """Get all validators for a specific epoch"""
    epoch = request.args.get('epoch', default=START_EPOCH, type=int)
    with_stake = request.args.get('with_stake', default='false', type=str).lower() == 'true'

    try:
        reader = Reader(epoch)
        validators = reader.get_validators(with_stake=with_stake)

        result = []
        for vote_key, v in validators.items():
            validator_data = {
                "voteAccountPubkey": v.voteAccountPubkey,
                "identityPubkey": v.identityPubkey,
                "name": v.name,
                "commission": v.commission,
                "activatedStake": v.activatedStake,
                "delinquent": v.delinquent,
                "skipRate": v.skipRate,
                "version": v.version,
                "valid": v.valid,
                # Info
                "website": v.website,
                "iconUrl": v.iconUrl,
                # Stakewiz data
                "stakewiz": v.stakewiz,
                "s_ip_city": v.s_ip_city,
                "s_ip_country": v.s_ip_country,
                "s_ip_latitude": v.s_ip_latitude,
                "s_ip_longitude": v.s_ip_longitude,
                "s_ip_asn": v.s_ip_asn,
                "s_ip_org": v.s_ip_org,
                "s_is_jito": v.s_is_jito,
                "s_staking_apy": v.s_staking_apy,
                "s_jito_apy": v.s_jito_apy,
                "s_total_apy": v.s_total_apy,
                # Marinade data
                "marinade": v.marinade,
                "m_dc_city": v.m_dc_city,
                "m_dc_country": v.m_dc_country,
                "m_dc_lat": v.m_dc_lat,
                "m_dc_lon": v.m_dc_lon,
                "m_node_ip": v.m_node_ip,
            }

            if with_stake:
                validator_data["staker_count"] = len(v.smap)
                validator_data["top_staker"] = v.top_staker()
                validator_data["stake_gini"] = float(v.stake_gini())

            result.append(validator_data)

        # Sort by activated stake descending
        result.sort(key=lambda x: x['activatedStake'], reverse=True)

        return jsonify({
            "epoch": epoch,
            "count": len(result),
            "validators": result
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/validators/<vote_account>', methods=['GET'])
def get_validator_detail(vote_account):
    """Get detailed information about a specific validator"""
    epoch = request.args.get('epoch', default=START_EPOCH, type=int)

    try:
        reader = Reader(epoch)
        validator = reader.get_validators(with_stake=True, validator=vote_account)

        if not validator:
            return jsonify({"error": "Validator not found"}), 404

        v = validator

        # Basic info
        result = {
            "voteAccountPubkey": v.voteAccountPubkey,
            "identityPubkey": v.identityPubkey,
            "name": v.name,
            "commission": v.commission,
            "activatedStake": v.activatedStake,
            "delinquent": v.delinquent,
            "skipRate": v.skipRate,
            "version": v.version,
            "valid": v.valid,
            "website": v.website,
            "iconUrl": v.iconUrl,
            "details": v.details,
            # Stakewiz
            "stakewiz": v.stakewiz,
            "s_ip_city": v.s_ip_city,
            "s_ip_country": v.s_ip_country,
            "s_ip_latitude": v.s_ip_latitude,
            "s_ip_longitude": v.s_ip_longitude,
            "s_ip_asn": v.s_ip_asn,
            "s_ip_org": v.s_ip_org,
            "s_is_jito": v.s_is_jito,
            "s_staking_apy": v.s_staking_apy,
            "s_jito_apy": v.s_jito_apy,
            "s_total_apy": v.s_total_apy,
            # Marinade
            "marinade": v.marinade,
            "m_dc_city": v.m_dc_city,
            "m_dc_country": v.m_dc_country,
            "m_dc_lat": v.m_dc_lat,
            "m_dc_lon": v.m_dc_lon,
            "m_node_ip": v.m_node_ip,
            "m_dc_asn": v.m_dc_asn,
            "m_dc_continent": v.m_dc_continent,
            # Stake analysis
            "staker_count": len(v.smap),
            "self_stake": v.self_stake(),
            "max_stake": v.max_stake(),
            "stake_gini": float(v.stake_gini()),
            "top_stakers": v.top_stakers(5),
            "top_n_ratio": [float(x) for x in v.top_n_staker_ratio(5)],
        }

        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/api/epochs', methods=['GET'])
def get_epochs():
    """Get available epoch range"""
    return jsonify({
        "start_epoch": START_EPOCH,
        "end_epoch": END_EPOCH,
        "current_default": START_EPOCH
    })

@app.route('/api/stats', methods=['GET'])
def get_stats():
    """Get overall statistics for an epoch"""
    epoch = request.args.get('epoch', default=START_EPOCH, type=int)

    try:
        reader = Reader(epoch)
        validators = reader.get_validators(with_stake=False)

        active_validators = [v for v in validators.values() if v.activatedStake > 0]
        nondelinquent = [v for v in validators.values() if not v.delinquent]
        total_stake = sum(v.activatedStake for v in validators.values())

        stakewiz_count = sum(1 for v in validators.values() if v.stakewiz)
        marinade_count = sum(1 for v in validators.values() if v.marinade)

        return jsonify({
            "epoch": epoch,
            "total_validators": len(validators),
            "active_validators": len(active_validators),
            "nondelinquent_validators": len(nondelinquent),
            "total_stake": total_stake,
            "stakewiz_count": stakewiz_count,
            "marinade_count": marinade_count
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, debug=True)
