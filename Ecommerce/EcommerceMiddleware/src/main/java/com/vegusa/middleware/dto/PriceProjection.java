package com.vegusa.middleware.dto;

import java.math.BigDecimal;

public interface PriceProjection {
    String getArticulo();
    BigDecimal getCost();
}
