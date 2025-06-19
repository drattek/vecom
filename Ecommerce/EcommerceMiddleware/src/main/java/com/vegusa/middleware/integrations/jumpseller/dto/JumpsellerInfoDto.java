package com.vegusa.middleware.integrations.jumpseller.dto;

public class JumpsellerInfoDto {
    private AppInfo store;

    public JumpsellerInfoDto() {}

    public JumpsellerInfoDto(AppInfo store) {
        this.store = store;
    }

    public AppInfo getStore() {
        return store;
    }

    public void setStore(AppInfo store) {
        this.store = store;
    }
}
