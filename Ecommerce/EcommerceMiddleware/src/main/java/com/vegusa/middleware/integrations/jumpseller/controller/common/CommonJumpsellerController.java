package com.vegusa.middleware.integrations.jumpseller.controller.common;

import com.vegusa.middleware.dto.ImportProductDTO;
import com.vegusa.middleware.integrations.jumpseller.dto.*;
import com.vegusa.middleware.integrations.jumpseller.service.common.CommonJumpsellerService;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class CommonJumpsellerController {
    @Autowired
    private CommonJumpsellerService commonJumpsellerService;

    @Autowired
    public CommonJumpsellerController() {}

    @GetMapping(value = "/app-info")
    public Mono<JumpsellerInfoDto> getAppInfo() {
        try {
            return commonJumpsellerService.getAppInfo();
        } catch (RuntimeException e){
            System.err.println(e.getMessage());
        }
        return null;
    }

    @GetMapping(value = "/app-language")
    public Mono<LanguageDto> getLanguage(){
        try {
            return commonJumpsellerService.getLanguage();
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    //@PostMapping(value = "/import-category")
    public void importCategory(@RequestBody HashMap<String, String> request){
        System.out.println("Inserting categories to database");
        String categories = request.get("categoryList");
        JSONArray categoryList = new JSONObject(categories).getJSONArray("content");

        commonJumpsellerService.importCategories(categoryList);
    }

    @PostMapping(value = "/mapped-brands")
    public void mappedBrands(@RequestBody List<MappedBrandDTO> data){
        commonJumpsellerService.mappedBrands(data);
    }

    @PostMapping(value = "/mapped-categories")
    public void mappedCategory(@RequestBody Map<String, Map<String, List<MappedCatergoryDTO>>> data){
        commonJumpsellerService.mappedCategories(data);
    }

    @PostMapping(value = "/update-categories")
    public Mono<Void> updateCategories(){
        commonJumpsellerService.syncCategories().subscribe();

        return Mono.empty();
    }

    @PostMapping(value = "/update-brands")
    public Mono<Void> updateBrands(){
        commonJumpsellerService.syncBrands().subscribe();

        return Mono.empty();
    }

    @PostMapping(value = "/resync-item")
    public JumpsellerProductDto resyncItem(@RequestBody Map<String, String> data){
        long id = Long.parseLong(data.get("id"));
        String internalCode = data.get("internal_code");
        return commonJumpsellerService.getSyncProduct(id, internalCode);
    }

    @PostMapping(value = "/update-titles")
    public void updateTitles(@RequestBody List<ImportProductDTO> data){
        commonJumpsellerService.updateTitles(data);
    }
}
