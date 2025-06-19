package com.vegusa.middleware.integrations.multivende.oauth;

import com.vegusa.middleware.entity.AuthTokenParameter;
import com.vegusa.middleware.integrations.multivende.dto.OAuthDto;
import com.vegusa.middleware.entity.Token;
import com.vegusa.middleware.repository.TokenRepository;
import com.vegusa.middleware.repository.AuthTokenParameterRepository;
import com.vegusa.middleware.utils.EncryptUtils;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.SecretKey;
import javax.crypto.spec.IvParameterSpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.time.Instant;

@Component
public class TokenStorageMultivende {
    private volatile String accessToken;
    private volatile String refreshToken;
    private volatile Instant expiresAt;
    private volatile Instant refreshTokenExpiresAt;
    private volatile String merchantId;
    private final String algorithm;
    private final TokenRepository tokenRepository;
    private final AuthTokenParameterRepository authTokenParameterRepository;

    @Autowired
    private EncryptUtils encryptUtils;

    public TokenStorageMultivende(TokenRepository tokenRepository, AuthTokenParameterRepository authTokenParameterRepository){
        this.tokenRepository = tokenRepository;
        this.authTokenParameterRepository = authTokenParameterRepository;
        this.algorithm = "AES/CBC/PKCS5Padding";
    }

    @PostConstruct
    public void init (){
        AuthTokenParameter parameter = authTokenParameterRepository.getTokenInfoParameters("MULTIVENDE");
        merchantId = parameter.getMerchantId();
        tokenRepository.findByIntegrationCompany("MULTIVENDE").ifPresent(token -> {
            try {
                SecretKey key = encryptUtils.convertStringToSecretKey(token.getSecretKey());
                IvParameterSpec ivParameterSpec = encryptUtils.convertStringToIvParameters(token.getInitializationVector());
                String refreshToken = encryptUtils.decrypt(algorithm, token.getCipherRefreshToken(), key, ivParameterSpec);
                String accessToken = encryptUtils.decrypt(algorithm, token.getCipherAccessToken(), key, ivParameterSpec);

                System.out.println("Token Multivende encontrado: " + accessToken);
                System.out.println("Refresh token Multivende encontrado: " + refreshToken);

                // Cargar la información de la BD para memorizar y tener un acceso rápido
                this.accessToken = accessToken;
                this.refreshToken = refreshToken;
                this.expiresAt = token.getAccessTokenExpiresAt().toInstant();
                this.refreshTokenExpiresAt = token.getRefreshTokenExpiresAt().toInstant();
            } catch (NoSuchPaddingException | IllegalBlockSizeException | BadPaddingException | NoSuchAlgorithmException | InvalidAlgorithmParameterException | InvalidKeyException e) {
                System.err.println("Error encontrando el token Multivende: " +  e.getMessage());
            }
        });
    }

    public String getAccessToken() {
        return accessToken;
    }

    public void setAccessToken(String accessToken) {
        this.accessToken = accessToken;
    }

    public String getRefreshToken() {
        return refreshToken;
    }

    public void setRefreshToken(String refreshToken) {
        this.refreshToken = refreshToken;
    }

    public Instant getExpiresAt() {
        return expiresAt;
    }

    public void setExpiresAt(Instant expiresAt) {
        this.expiresAt = expiresAt;
    }

    public Instant getRefreshTokenExpiresAt() {
        return refreshTokenExpiresAt;
    }

    public void setRefreshTokenExpiresAt(Instant refreshTokenExpiresAt) {
        this.refreshTokenExpiresAt = refreshTokenExpiresAt;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public boolean isAccessTokenExpired(){
        return this.expiresAt == null | Instant.now().isAfter(this.expiresAt.minusSeconds(60));
    }

    public boolean isRefreshTokenExpired(){
        return this.refreshTokenExpiresAt == null | Instant.now().isAfter(this.refreshTokenExpiresAt);
    }

    public void save(OAuthDto request){
        try {
            SecretKey key = encryptUtils.generateKey(128);
            IvParameterSpec ivParameterSpec = encryptUtils.generateIv();
            String cipherAccessToken = encryptUtils.encrypt(algorithm, request.getToken(), key, ivParameterSpec);
            String cipherRefreshToken = encryptUtils.encrypt(algorithm, request.getRefreshToken(), key, ivParameterSpec);
            SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'");

            // Guardar la información en BD actualizar/crear
            Token token = tokenRepository.findByIntegrationCompany("MULTIVENDE")
                    .orElseGet(() -> {
                        Token new_token = new Token();
                        new_token.setIntegrationCompany("MULTIVENDE");
                        return new_token;
                    });
            token.setResponseId(request.getId());
            token.setStatus(request.getStatus());
            token.setCipherAccessToken(cipherAccessToken);
            token.setAccessTokenExpiresAt(formatter.parse(request.getExpiresAt().toString()));
            token.setCipherRefreshToken(cipherRefreshToken);
            token.setRefreshTokenExpiresAt(formatter.parse(request.getRefreshTokenExpiresAt().toString()));
            token.setSecretKey(encryptUtils.convertSecretKeyToString(key));
            token.setInitializationVector(encryptUtils.convertIvParametersToString(ivParameterSpec));
            token.setUpdatedAt(formatter.parse(request.getUpdatedAt()));
            token.setCreatedAt(formatter.parse(request.getCreatedAt()));
            token.setIntegrationCompany("MULTIVENDE");

            tokenRepository.save(token);

            // Memorizar para acceso rápido de la información
            this.accessToken = request.getToken();
            this.refreshToken = request.getRefreshToken();
            this.expiresAt = request.getExpiresAt();
            this.refreshTokenExpiresAt = request.getRefreshTokenExpiresAt();
            System.out.println("Token expires at: " + request.getExpiresAt());
            System.out.println("Token Multivende saved successfully");
        } catch (NoSuchAlgorithmException | ParseException | InvalidKeyException | BadPaddingException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException e) {
            System.err.println("Could not save token: " + e.getMessage());
        }
    }
}
