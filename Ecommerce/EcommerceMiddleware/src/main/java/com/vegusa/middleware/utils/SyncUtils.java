package com.vegusa.middleware.utils;

import com.vegusa.middleware.entity.Products;
import org.springframework.stereotype.Component;

import java.util.Objects;

@Component
public class SyncUtils {

    public String getName(Products product, String shortDescription) {
        String name = shortDescription;
        boolean hasBrand = name.contains(product.getBrand());
        boolean hasPartNumber = name.contains(product.getPartNumber());
        if(!hasBrand && !Objects.equals(product.getBrand(), "")){
            name = !Objects.equals(name, "") ? name + " "  + product.getBrand() : product.getBrand();
        }
        if(!hasPartNumber && !Objects.equals(product.getPartNumber(), "")){
            name = !Objects.equals(name, "") ? name + " " + product.getPartNumber() : product.getPartNumber();
        }
        return capitalize(name);
    }

    public String getDescription(Products product, String name){
        if (!product.getMetaDescription().isBlank()) {
            return product.getMetaDescription();
        }

        String prefix = "-- Tienda Vegusa Maquinaria, Distribuidor Autorizado Unicarriers, Bobcat, JLG, Flexi. -- ";

        return prefix + name + " - " + product.getItemId();
    }

    private String capitalize(String str) {
        if (str == null || str.isEmpty()) {
            return str;
        }
        return str.substring(0, 1).toUpperCase() + str.substring(1).toLowerCase();
    }
}
