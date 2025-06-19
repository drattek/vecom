package com.vegusa.middleware.integrations.jumpseller.dto;

import java.math.BigDecimal;

public interface StockProjection {
    String getArticulo();
    BigDecimal getTotal();
}
