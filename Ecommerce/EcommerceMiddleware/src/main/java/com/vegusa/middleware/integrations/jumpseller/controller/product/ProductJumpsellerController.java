package com.vegusa.middleware.integrations.jumpseller.controller.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.dto.AttributeCompatibility;
import com.vegusa.middleware.entity.ProductAttributeValues;
import com.vegusa.middleware.entity.Products;
import com.vegusa.middleware.integrations.jumpseller.service.product.ProductJumpsellerService;
import com.vegusa.middleware.repository.local.ProductAttributeValuesRepository;
import com.vegusa.middleware.repository.local.ProductsRepository;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.HashMap;
import java.util.List;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class ProductJumpsellerController {

    @Autowired
    private ProductJumpsellerService productJumpsellerService;

    @Autowired
    private ProductsRepository productsRepository;

    @Autowired
    private ProductAttributeValuesRepository productAttributeValuesRepository;

    @Autowired
    public ProductJumpsellerController(){}

    @GetMapping(value = "/get-products")
    public Mono<ResponseEntity<String>> getProducts(){
        try {
            System.out.println("Get all products");
            productJumpsellerService.getAllProducts().subscribe();

            return Mono.just(ResponseEntity.accepted().body("Getting all products"));
        } catch (RuntimeException e){
            System.err.println("Error getting all products");
        }
        return null;
    }

    @PostMapping(value = "/sync-prices")
    public Mono<ResponseEntity<String>> updatePricesJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start updating prices");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    products = MWUtils.bodyValidation(request.get("productList")); // Read the array of products uploaded
            JSONArray productsList = new JSONObject(products).getJSONArray("content"); // Transform the productlist to JsonArray
            productJumpsellerService.updatePrices(productsList, dataAreaId).subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/sync-brands")
    public Mono<ResponseEntity<String>> updateBrands(){
        try {
            System.out.println("Start updating brands");
            productJumpsellerService.syncBrands().subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e){
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/sync-products")
    public Mono<ResponseEntity<String>> synProductJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start sync products");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId"));
            productJumpsellerService.updateProducts().subscribe();

            return Mono.just(ResponseEntity.accepted().body("Syn started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/update-title")
    public Mono<Void> updateTitles(){
        productJumpsellerService.changeTitle().subscribe();

        return Mono.empty();
    }

    // Custom field
    @GetMapping(value = "/get-custom-field")
    public Mono<String> getCustomFields(@RequestParam("item_id") String itemId){
        return productJumpsellerService.getCustomFields(Long.parseLong(itemId));
    }

    @GetMapping(value = "/syn-attribute")
    public Mono<Void> synAttribute(){
        productJumpsellerService.updateAttributes().subscribe();

        return Mono.empty();
    }

    @GetMapping(value = "/model-compatibility")
    public Void modelCompatibility(){
        productJumpsellerService.setCompatibility();

        return null;
    }

    @PostMapping("/set-compatibility")
    public Void insertAttributes(@RequestBody List<AttributeCompatibility> attributes){
        for (AttributeCompatibility attribute : attributes){
            if (attribute.getCompatibility() != null && !attribute.getCompatibility().isEmpty()){
                System.out.println("Compatibility: " + attribute.getCompatibility());
                System.out.println("Part number: " + attribute.getPartNumber());
                Products product = productsRepository.getProductByNumber(attribute.getPartNumber(), DataArea.MSB.name())
                        .orElseGet(() -> null);

                if (product != null){
                    ProductAttributeValues newAttribute = new ProductAttributeValues();
                    newAttribute.setItemId(product.getItemId());
                    newAttribute.setInterfaceId("DYN");
                    newAttribute.setProductAttributeId("MODEL_ASSINGMENT");
                    newAttribute.setValue(attribute.getCompatibility());
                    newAttribute.setSkipNull("TRUE");
                    newAttribute.setCreatedAt(Instant.now());
                    newAttribute.setUpdatedAt(Instant.now());
                    newAttribute.setDataAreaId("MSB");
                    newAttribute.setInterfaceRefRecId(4L);
                    newAttribute.setProductAttributeRefRecId(23L);
                    newAttribute.setCompanyRefRecId(1L);
                    newAttribute.setProductRefRecId(product.getId());

                    productAttributeValuesRepository.save(newAttribute);
                    System.out.println("Save: " + product.getItemId());
                }
            }
        }

        return null;
    }
}
