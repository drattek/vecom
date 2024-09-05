package com.vegusa.oauth2_0.encrypt_decrypt;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.SecretKey;
import javax.crypto.spec.IvParameterSpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;

public interface EncryptDecryptInterface
{
    SecretKey generateKey(int n) throws NoSuchAlgorithmException;

    IvParameterSpec generateIv();

    String encrypt(String algorithm, String input, SecretKey key, IvParameterSpec iv)
            throws NoSuchPaddingException, NoSuchAlgorithmException, InvalidAlgorithmParameterException,
                InvalidKeyException, BadPaddingException, IllegalBlockSizeException;

    String decrypt(String algorithm, String cipherText, SecretKey key, IvParameterSpec iv)
            throws NoSuchPaddingException, NoSuchAlgorithmException, InvalidAlgorithmParameterException,
                InvalidKeyException, BadPaddingException, IllegalBlockSizeException;
}
