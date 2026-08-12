package com.vegusa.ecommerce.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.ecommerce.dto.EcomProductDTO;
import com.vegusa.ecommerce.dto.ItemInventLocationDTO;
import com.vegusa.ecommerce.dto.NissanExistenciasDTO;
import lombok.RequiredArgsConstructor;
import org.springframework.data.redis.core.RedisCallback;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.data.redis.serializer.RedisSerializer;
import org.springframework.stereotype.Service;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Objects;

@Service
@RequiredArgsConstructor
public class RedisProductService {
    private final RedisTemplate<String, String> redisTemplate;
    private final ObjectMapper objectMapper;

    public void saveProducts(List<EcomProductDTO> products){
        redisTemplate.executePipelined(
                (RedisCallback<Object>) connection -> {
                    for (EcomProductDTO product : products){
                        try {
                            String key = "product:" + product.code();
                            String json = objectMapper.writeValueAsString(product);

                            byte[] redisKey = key.getBytes(StandardCharsets.UTF_8);
                            byte[] redisValue =  json.getBytes(StandardCharsets.UTF_8);

                            connection.stringCommands()
                                    .set(redisKey, redisValue);
                        } catch (JsonProcessingException ex){
                            throw new RuntimeException(ex);
                        }
                    }
                    return null;
                }
        );
    }

    public void saveStock(List<ItemInventLocationDTO> products){
        redisTemplate.executePipelined(
                (RedisCallback<Object>)connection -> {
                    for (ItemInventLocationDTO product : products){
                        try {
                            String key = "stock:" + product.articulo() + "-" + product.almacen();

                            String json = objectMapper.writeValueAsString(product);

                            byte[] redisKey = key.getBytes(StandardCharsets.UTF_8);
                            byte[] redisValue = json.getBytes(StandardCharsets.UTF_8);

                            connection.stringCommands()
                                    .set(redisKey, redisValue);
                        } catch (JsonProcessingException ex){
                            throw new RuntimeException(ex);
                        }
                    }
                    return null;
                }
        );
    }

    public void saveExistencias(List<NissanExistenciasDTO> existencias){
        redisTemplate.executePipelined(
                (RedisCallback<Object>) connection -> {
                    for (NissanExistenciasDTO existencia : existencias){
                        try {
                            // La clave incluye la agencia (= almacén) porque el ERP devuelve una fila
                            // por SKU+agencia: usar solo el código sobrescribía el stock de todas las
                            // agencias menos la última procesada para cada SKU.
                            String key = "nissan:existencia:" + existencia.code() + "-" + existencia.agencyName();
                            String json = objectMapper.writeValueAsString(existencia);

                            byte[] redisKey = key.getBytes(StandardCharsets.UTF_8);
                            byte[] redisValue = json.getBytes(StandardCharsets.UTF_8);

                            connection.stringCommands()
                                    .set(redisKey, redisValue);
                        } catch (JsonProcessingException ex){
                            throw new RuntimeException(ex);
                        }
                    }
                    return null;
                }
        );
    }
}
