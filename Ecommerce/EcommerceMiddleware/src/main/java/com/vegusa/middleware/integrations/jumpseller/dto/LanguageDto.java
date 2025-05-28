package com.vegusa.middleware.integrations.jumpseller.dto;

public class LanguageDto {
    private Language[] languages;

    private static class Language {
        private String code;
        private String name;

        public Language(String code, String name) {
            this.code = code;
            this.name = name;
        }

        public Language() {}

        public String getCode() {
            return code;
        }

        public void setCode(String code) {
            this.code = code;
        }

        public String getName() {
            return name;
        }

        public void setName(String name) {
            this.name = name;
        }
    }

    public LanguageDto() {
    }

    public LanguageDto(Language[] languages) {
        this.languages = languages;
    }

    public Language[] getLanguages() {
        return languages;
    }

    public void setLanguages(Language[] languages) {
        this.languages = languages;
    }
}
