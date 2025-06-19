package com.vegusa.middleware.integrations.multivende.utils;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.databind.*;
import com.vegusa.middleware.integrations.multivende.dto.ImageReference;

import java.io.IOException;
import java.util.*;

public class ImageReferenceDeserializer extends JsonDeserializer<List<ImageReference>> {

    @Override
    public List<ImageReference> deserialize(JsonParser p, DeserializationContext ctxt) throws IOException {
        List<ImageReference> result = new ArrayList<>();
        ObjectMapper mapper = (ObjectMapper) p.getCodec();

        if (p.currentToken() == JsonToken.START_ARRAY) {
            while (p.nextToken() != JsonToken.END_ARRAY) {
                JsonNode node = mapper.readTree(p);
                if (node.isTextual()) {
                    result.add(new ImageReference(node.asText()));
                } else if (node.has("id")) {
                    result.add(new ImageReference(node.get("id").asText()));
                }
            }
        }

        return result;
    }
}
