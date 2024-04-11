package com.vegusa.veg_mv_integration_midd.oauth2_0.utils;

public final class AuthUtils
{
    private AuthUtils(){}

    public static Integer getMaxAttemptsToGetAccessToken(){ return 5; }
    public static Integer getMaxAttemptsToGetRefreshToken(){ return 5; }

    public static Integer getThreadSleepErrorAccessToken(){ return 60000; }

    public static Integer getThreadSleepErrorRefreshToken(){ return 300000; }




}
