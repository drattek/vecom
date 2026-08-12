package com.vegusa.ecommerce.controller;

import com.vegusa.ecommerce.service.ProductService;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

// Rutas de disparo manual para pruebas: ejecutan de forma síncrona las mismas
// funciones que corren via @Scheduled en ProductService (ver sync.*.cron-expression
// en application.yml). No forman parte del flujo oficial ERP -> Synapse Bridge ->
// Redis/RabbitMQ -> Core, son solo para verificar que el sync funciona sin esperar el cron.
@Controller
@RestController
@RequestMapping("/products")
public class ProductController {
    private final ProductService productService;

    public ProductController(ProductService productService){
        this.productService = productService;
    }

    @GetMapping("/items")
    public void saveItems(){
        productService.getItems();
    }

    @GetMapping("/stock")
    public void saveStock(){
        productService.getStockInfo();
    }

    @GetMapping("/existencias")
    public void saveExistencias(){
        productService.getExistenciasInfo();
    }
}
