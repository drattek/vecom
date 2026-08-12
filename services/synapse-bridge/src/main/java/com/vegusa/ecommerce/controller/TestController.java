package com.vegusa.ecommerce.controller;

import com.vegusa.ecommerce.dto.ItemInventLocationDTO;
import com.vegusa.ecommerce.repository.ItemInventLocationRepository;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;

@Controller
@RestController
public class TestController {
    private final ItemInventLocationRepository itemInventLocationRepository;

    public TestController(ItemInventLocationRepository itemInventLocationRepository) {
        this.itemInventLocationRepository = itemInventLocationRepository;
    }

    @GetMapping("/test")
    public List<ItemInventLocationDTO> getStock(){
        int pageSize = 1000;
        int offset = 0;

        return itemInventLocationRepository.getLocations(offset, pageSize);
    }
}
