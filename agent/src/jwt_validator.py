"""
JWT Token Validation for TitanCompute Agent
Validates JWT tokens issued by the coordinator for secure client-agent communication
"""

import jwt
import logging
import time
from typing import Optional, Dict, Any
from dataclasses import dataclass


@dataclass
class JWTClaims:
    """Validated JWT claims"""
    agent_id: str
    client_id: str
    model: str
    jti: str  # JWT ID
    iat: int  # Issued at
    exp: int  # Expires at
    nbf: int  # Not before
    iss: str  # Issuer


class JWTValidator:
    """Validates JWT tokens from the coordinator"""
    
    def __init__(self):
        self.logger = logging.getLogger(__name__)
        self.public_key = None
        self.algorithm = "RS256"
        self.issuer = "titancompute-coordinator"
        
    def set_public_key(self, public_key_pem: str):
        """Set the coordinator's public key for validation"""
        try:
            self.public_key = jwt.algorithms.RSAAlgorithm.from_jwk(
                jwt.api_jwk.PyJWK.from_pem(public_key_pem.encode()).key
            )
            self.logger.info("JWT public key configured successfully")
        except Exception as e:
            self.logger.error(f"Failed to configure JWT public key: {e}")
            raise
    
    def validate_token(self, token: str, expected_agent_id: str) -> Optional[JWTClaims]:
        """Validate a JWT token"""
        if not self.public_key:
            self.logger.error("JWT public key not configured")
            return None
            
        try:
            # Decode and validate token
            payload = jwt.decode(
                token,
                self.public_key,
                algorithms=[self.algorithm],
                issuer=self.issuer,
                options={
                    "verify_signature": True,
                    "verify_exp": True,
                    "verify_nbf": True,
                    "verify_iat": True,
                    "verify_iss": True,
                    "require": ["agent_id", "client_id", "model", "jti", "iat", "exp", "nbf", "iss"]
                }
            )
            
            # Validate agent_id matches this agent
            if payload.get("agent_id") != expected_agent_id:
                self.logger.warning(f"Token agent_id mismatch: expected {expected_agent_id}, got {payload.get('agent_id')}")
                return None
            
            # Create claims object
            claims = JWTClaims(
                agent_id=payload["agent_id"],
                client_id=payload["client_id"],
                model=payload["model"],
                jti=payload["jti"],
                iat=payload["iat"],
                exp=payload["exp"],
                nbf=payload["nbf"],
                iss=payload["iss"]
            )
            
            self.logger.debug(f"Successfully validated token for client {claims.client_id}")
            return claims
            
        except jwt.ExpiredSignatureError:
            self.logger.warning("Token has expired")
            return None
        except jwt.InvalidTokenError as e:
            self.logger.warning(f"Invalid token: {e}")
            return None
        except Exception as e:
            self.logger.error(f"Token validation failed: {e}")
            return None
    
    def is_token_expired(self, token: str) -> bool:
        """Check if a token is expired without full validation"""
        try:
            # Decode without verification to check expiration
            payload = jwt.decode(token, options={"verify_signature": False})
            exp = payload.get("exp")
            if exp:
                return time.time() > exp
            return True
        except Exception:
            return True
    
    def extract_claims_unsafe(self, token: str) -> Optional[Dict[str, Any]]:
        """Extract claims without validation (for debugging)"""
        try:
            return jwt.decode(token, options={"verify_signature": False})
        except Exception as e:
            self.logger.warning(f"Failed to extract claims: {e}")
            return None
